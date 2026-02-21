package repository

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

func extendRedisTTL(ctx context.Context, redisClient *redis.Redis, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	if redisClient == nil {
		return errors.New("cache is nil")
	}

	currentTTL, err := redisClient.TtlCtx(ctx, key)
	if err != nil {
		return redisClient.ExpireCtx(ctx, key, int(ttl/time.Second))
	}
	if currentTTL < 0 || ttl > time.Duration(currentTTL)*time.Second {
		return redisClient.ExpireCtx(ctx, key, int(ttl/time.Second))
	}
	return nil
}
