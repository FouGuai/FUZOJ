package svc

import (
	"fuzoj/services/rank_rpc_service/internal/config"
	"fuzoj/services/rank_rpc_service/internal/repository"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type ServiceContext struct {
	Config          config.Config
	Redis           *redis.Redis
	LeaderboardRepo *repository.LeaderboardRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	redisClient := redis.MustNewRedis(c.RankRedis)
	repo := repository.NewLeaderboardRepository(redisClient, c.Rank.PageCacheTTL, c.Rank.EmptyTTL)
	return &ServiceContext{
		Config:          c,
		Redis:           redisClient,
		LeaderboardRepo: repo,
	}
}
