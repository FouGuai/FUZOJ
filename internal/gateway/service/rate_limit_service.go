package service

import (
	"context"
	"fmt"
	"time"

	"fuzoj/internal/common/cache"
	pkgerrors "fuzoj/pkg/errors"
)

// RateLimitService enforces fixed-window limits using Redis.
type RateLimitService struct {
	cache        cache.BasicOps
	window       time.Duration
	redisTimeout time.Duration
}

func NewRateLimitService(cacheClient cache.BasicOps, window time.Duration, redisTimeout time.Duration) *RateLimitService {
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

	acquired, err := s.cache.SetNX(ctxCache, key, 1, window)
	if err != nil {
		return pkgerrors.Wrapf(err, pkgerrors.CacheError, "rate limit check failed")
	}
	var count int64
	if acquired {
		count = 1
	} else {
		count, err = s.cache.Incr(ctxCache, key)
		if err != nil {
			return pkgerrors.Wrapf(err, pkgerrors.CacheError, "rate limit check failed")
		}
		ttl, ttlErr := s.cache.TTL(ctxCache, key)
		if ttlErr == nil && ttl <= 0 {
			_ = s.cache.Expire(ctxCache, key, window)
		}
	}
	if int(count) > max {
		return pkgerrors.New(pkgerrors.TooManyRequests).WithMessage(fmt.Sprintf("rate limit exceeded for %s", key))
	}
	return nil
}
