package repository

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// TokenBlacklistRepository checks token revocation with Redis + local cache.
type TokenBlacklistRepository struct {
	local        *LRUCache
	redis        *redis.Redis
	localTTL     time.Duration
}

func NewTokenBlacklistRepository(local *LRUCache, redisClient *redis.Redis, localTTL time.Duration) *TokenBlacklistRepository {
	return &TokenBlacklistRepository{
		local:        local,
		redis:        redisClient,
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
	blacklisted, err := r.redis.SismemberCtx(ctx, tokenBlacklistKey, tokenHash)
	if err != nil {
		return false, err
	}
	if blacklisted && r.local != nil {
		r.local.Set(tokenHash, true, r.localTTL)
	}
	return blacklisted, nil
}
