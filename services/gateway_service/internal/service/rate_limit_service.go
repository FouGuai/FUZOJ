package service

import (
	"context"
	"fmt"
	"time"

	pkgerrors "fuzoj/pkg/errors"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// RateLimitService enforces fixed-window limits using Redis.
type RateLimitService struct {
	cache        *redis.Redis
	window       time.Duration
	redisTimeout time.Duration
}

func NewRateLimitService(cacheClient *redis.Redis, window time.Duration, redisTimeout time.Duration) *RateLimitService {
	return &RateLimitService{cache: cacheClient, window: window, redisTimeout: redisTimeout}
}

func (s *RateLimitService) Allow(ctx context.Context, key string, max int, window time.Duration) error {
	if s.cache == nil {
		return pkgerrors.New(pkgerrors.ServiceUnavailable).WithMessage("rate limit cache is unavailable")
	}
	if max <= 0 {
		return nil
	}
	if window <= 0 {
		window = s.window
	}

	ctxCache, cancel := context.WithTimeout(ctx, s.redisTimeout)
	defer cancel()

	acquired, err := s.cache.SetnxExCtx(ctxCache, key, "1", seconds(window))
	if err != nil {
		return pkgerrors.Wrapf(err, pkgerrors.CacheError, "rate limit check failed")
	}
	var count int64
	if acquired {
		count = 1
	} else {
		count, err = s.cache.IncrCtx(ctxCache, key)
		if err != nil {
			return pkgerrors.Wrapf(err, pkgerrors.CacheError, "rate limit check failed")
		}
		ttl, ttlErr := s.cache.TtlCtx(ctxCache, key)
		if ttlErr == nil && ttl <= 0 {
			_ = s.cache.ExpireCtx(ctxCache, key, seconds(window))
		}
	}
	if int(count) > max {
		return pkgerrors.New(pkgerrors.TooManyRequests).WithMessage(fmt.Sprintf("rate limit exceeded for %s", key))
	}
	return nil
}

func seconds(window time.Duration) int {
	value := int(window.Seconds())
	if value <= 0 {
		return 1
	}
	return value
}
