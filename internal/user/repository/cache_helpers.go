package repository

import (
	"context"
	"time"

	"fuzoj/internal/common/cache"
)

func extendTTL(ctx context.Context, cacheClient cache.Cache, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	currentTTL, err := cacheClient.TTL(ctx, key)
	if err != nil {
		return cacheClient.Expire(ctx, key, ttl)
	}

	if currentTTL < 0 || ttl > currentTTL {
		return cacheClient.Expire(ctx, key, ttl)
	}

	return nil
}
