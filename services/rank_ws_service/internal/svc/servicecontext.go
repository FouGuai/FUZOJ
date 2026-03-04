package svc

import (
	"fuzoj/services/rank_ws_service/internal/config"
	"fuzoj/services/rank_ws_service/internal/repository"
	"fuzoj/services/rank_ws_service/internal/ws"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type ServiceContext struct {
	Config          config.Config
	Redis           *redis.Redis
	PubSubClient    *red.Client
	LeaderboardRepo *repository.LeaderboardRepository
	Hub             *ws.Hub
}

func NewServiceContext(c config.Config) *ServiceContext {
	redisClient := redis.MustNewRedis(c.Redis)
	pubsubClient := newPubSubClient(c.Redis)
	repo := repository.NewLeaderboardRepository(redisClient, c.Rank.PageCacheTTL, c.Rank.EmptyTTL)
	hub := ws.NewHub(repo, pubsubClient, c.Rank.WSDebounce)

	return &ServiceContext{
		Config:          c,
		Redis:           redisClient,
		PubSubClient:    pubsubClient,
		LeaderboardRepo: repo,
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
		Username: conf.User,
		Password: conf.Pass,
	}
	return red.NewClient(opt)
}
