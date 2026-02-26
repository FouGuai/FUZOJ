package svc

import (
	"time"

	"fuzoj/services/gateway_service/internal/config"
	"fuzoj/services/gateway_service/internal/discovery"
	"fuzoj/services/gateway_service/internal/repository"
	"fuzoj/services/gateway_service/internal/service"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/queue"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type ServiceContext struct {
	Config        config.Config
	AuthService   *service.AuthService
	RateService   *service.RateLimitService
	BanRepo       *repository.BanCacheRepository
	BlacklistRepo *repository.TokenBlacklistRepository
	MQClient      queue.MessageQueue
	RedisClient   *redis.Redis
	Registry      *discovery.RegistryManager
}

func NewServiceContext(cfg config.Config) (*ServiceContext, error) {
	redisClient, err := redis.NewRedis(cfg.Redis)
	if err != nil {
		return nil, err
	}

	banLocal := repository.NewLRUCache(cfg.Cache.BanLocalSize, cfg.Cache.BanLocalTTL)
	banRepo := repository.NewBanCacheRepository(banLocal, redisClient, cfg.Cache.BanLocalTTL)
	tokenLocal := repository.NewLRUCache(tokenCacheSize(cfg.Cache.BanLocalSize), cfg.Cache.TokenBlacklistCacheTTL)
	blacklistRepo := repository.NewTokenBlacklistRepository(tokenLocal, redisClient, cfg.Cache.TokenBlacklistCacheTTL)

	authService := service.NewAuthService(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, blacklistRepo, banRepo)
	redisTimeout := cfg.Redis.PingTimeout
	if redisTimeout <= 0 {
		redisTimeout = time.Second
	}
	rateService := service.NewRateLimitService(redisClient, cfg.Rate.Window, redisTimeout)

	ctx := &ServiceContext{
		Config:        cfg,
		AuthService:   authService,
		RateService:   rateService,
		BanRepo:       banRepo,
		BlacklistRepo: blacklistRepo,
		RedisClient:   redisClient,
	}

	registry, err := discovery.NewRegistryManager(cfg.Bootstrap.Etcd)
	if err != nil {
		return nil, err
	}
	ctx.Registry = registry

	if cfg.BanEvent.Enabled {
		mqClient, mqErr := kq.NewQueue(cfg.BuildKqConf(), service.NewBanEventHandler(banRepo, cfg.Cache.BanLocalTTL))
		if mqErr != nil {
			registry.Close()
			return nil, mqErr
		}
		ctx.MQClient = mqClient
	}

	return ctx, nil
}

func (s *ServiceContext) Close() {
	if s.MQClient != nil {
		s.MQClient.Stop()
	}
	if s.Registry != nil {
		s.Registry.Close()
	}
}

func tokenCacheSize(banCacheSize int) int {
	size := banCacheSize / 10
	if size < 1024 {
		size = 1024
	}
	return size
}
