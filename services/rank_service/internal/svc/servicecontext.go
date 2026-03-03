package svc

import (
	"fuzoj/services/rank_service/internal/config"
	"fuzoj/services/rank_service/internal/consumer"
	"fuzoj/services/rank_service/internal/repository"
	"fuzoj/services/rank_service/internal/ws"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type ServiceContext struct {
	Config          config.Config
	Redis           *redis.Redis
	PubSubClient    *red.Client
	LeaderboardRepo *repository.LeaderboardRepository
	UpdateBatcher   *consumer.UpdateBatcher
	UpdateQueue     queue.MessageQueue
	Hub             *ws.Hub
}

func NewServiceContext(c config.Config) *ServiceContext {
	redisClient := redis.MustNewRedis(c.Redis)
	pubsubClient := newPubSubClient(c.Redis)
	repo := repository.NewLeaderboardRepository(redisClient, c.Rank.PageCacheTTL, c.Rank.EmptyTTL)
	batcher := consumer.NewUpdateBatcher(repo, pubsubClient, c.Rank.BatchSize, c.Rank.BatchInterval)
	hub := ws.NewHub(repo, pubsubClient, c.Rank.WSDebounce)

	var updateQueue queue.MessageQueue
	if len(c.Kafka.Brokers) > 0 && c.Rank.UpdateTopic != "" {
		updateConsumer := consumer.NewRankUpdateConsumer(batcher, c.Timeouts.MQ)
		kqConf := consumer.BuildRankUpdateKqConf(c)
		updateQueue = kq.MustNewQueue(kqConf, updateConsumer)
	}

	return &ServiceContext{
		Config:          c,
		Redis:           redisClient,
		PubSubClient:    pubsubClient,
		LeaderboardRepo: repo,
		UpdateBatcher:   batcher,
		UpdateQueue:     updateQueue,
		Hub:             hub,
	}
}

func newPubSubClient(conf redis.RedisConf) *red.Client {
	if conf.Host == "" {
		return nil
	}
	if conf.Type != "" && conf.Type != "node" {
		logx.Errorf("redis pubsub only supports node type, got %s", conf.Type)
		return nil
	}
	opt := &red.Options{
		Addr:     conf.Host,
		Password: conf.Pass,
		DB:       conf.DB,
	}
	return red.NewClient(opt)
}
