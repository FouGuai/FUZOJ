package svc

import (
	"crypto/tls"
	"fuzoj/services/rank_service/internal/config"
	"fuzoj/services/rank_service/internal/consumer"
	"fuzoj/services/rank_service/internal/repository"
	"fuzoj/services/rank_service/internal/worker"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config          config.Config
	Conn            sqlx.SqlConn
	Redis           *redis.Redis
	PubSubClient    *red.Client
	LeaderboardRepo *repository.LeaderboardRepository
	SnapshotRepo    *repository.SnapshotRepository
	Snapshotter     *worker.Snapshotter
	UpdateBatcher   *consumer.UpdateBatcher
	UpdateQueue     queue.MessageQueue
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	redisClient := redis.MustNewRedis(c.Redis)
	pubsubClient := newPubSubClient(c.Redis)
	repo := repository.NewLeaderboardRepository(redisClient, c.Rank.PageCacheTTL, c.Rank.EmptyTTL)
	batcher := consumer.NewUpdateBatcher(repo, pubsubClient, c.Rank.BatchSize, c.Rank.BatchInterval, c.Timeouts.MQ)
	snapshotRepo := repository.NewSnapshotRepository(conn)
	snapshotter := worker.NewSnapshotter(
		snapshotRepo,
		repo,
		redisClient,
		c.Rank.SnapshotInterval,
		c.Rank.SnapshotPageSize,
		c.Rank.SnapshotBatch,
		c.Timeouts.Cache,
		c.Timeouts.DB,
		c.Rank.RecoverOnStart,
	)

	var updateQueue queue.MessageQueue
	if len(c.Kafka.Brokers) > 0 && c.Rank.UpdateTopic != "" {
		updateConsumer := consumer.NewRankUpdateConsumer(batcher, c.Timeouts.MQ)
		kqConf := consumer.BuildRankUpdateKqConf(c)
		updateQueue = kq.MustNewQueue(kqConf, updateConsumer)
	}

	return &ServiceContext{
		Config:          c,
		Conn:            conn,
		Redis:           redisClient,
		PubSubClient:    pubsubClient,
		LeaderboardRepo: repo,
		SnapshotRepo:    snapshotRepo,
		Snapshotter:     snapshotter,
		UpdateBatcher:   batcher,
		UpdateQueue:     updateQueue,
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
		Username: conf.User,
		Password: conf.Pass,
		TLSConfig: func() *tls.Config {
			if !conf.Tls {
				return nil
			}
			return &tls.Config{}
		}(),
	}
	return red.NewClient(opt)
}
