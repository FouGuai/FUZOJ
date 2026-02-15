package repository

import (
	"context"
	"errors"
	"time"

	"fuzoj/internal/common/cache"
)

// TokenBlacklistRepository checks token revocation with Redis + local cache.
type TokenBlacklistRepository struct {
	local        *LRUCache
	redis        cache.SetOps
	redisTimeout time.Duration
	localTTL     time.Duration
}

func NewTokenBlacklistRepository(local *LRUCache, redis cache.SetOps, redisTimeout time.Duration, localTTL time.Duration) *TokenBlacklistRepository {
	return &TokenBlacklistRepository{
		local:        local,
		redis:        redis,
		redisTimeout: redisTimeout,
		localTTL:     localTTL,
	}
}

func (r *TokenBlacklistRepository) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	if tokenHash == "" {
		return false, nil
	}
	if r.local != nil {
		if val, ok := r.local.Get(tokenHash); ok {
			return val, nil
		}
	}
	if r.redis == nil {
		return false, errors.New("redis is nil")
	}
	ctxCache, cancel := context.WithTimeout(ctx, r.redisTimeout)
	defer cancel()
	blacklisted, err := r.redis.SIsMember(ctxCache, tokenBlacklistKey, tokenHash)
	if err != nil {
		return false, err
	}
	if blacklisted && r.local != nil {
		r.local.Set(tokenHash, true, r.localTTL)
	}
	return blacklisted, nil
}
