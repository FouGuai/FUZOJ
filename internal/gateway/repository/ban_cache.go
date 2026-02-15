package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/cache"
)

// BanCacheRepository checks banned users with local cache + Redis.
type BanCacheRepository struct {
	local       *LRUCache
	redis       cache.SetOps
	redisTTL    time.Duration
	redisTimeout time.Duration
}

func NewBanCacheRepository(local *LRUCache, redis cache.SetOps, redisTimeout time.Duration, ttl time.Duration) *BanCacheRepository {
	return &BanCacheRepository{
		local:        local,
		redis:        redis,
		redisTimeout: redisTimeout,
		redisTTL:     ttl,
	}
}

func (r *BanCacheRepository) IsBanned(ctx context.Context, userID int64) (bool, error) {
	key := fmt.Sprintf("%d", userID)
	if r.local != nil {
		if val, ok := r.local.Get(key); ok {
			return val, nil
		}
	}
	if r.redis == nil {
		return false, errors.New("redis is nil")
	}
	ctxCache, cancel := context.WithTimeout(ctx, r.redisTimeout)
	defer cancel()
	banned, err := r.redis.SIsMember(ctxCache, userBannedKey, userID)
	if err != nil {
		return false, err
	}
	if banned && r.local != nil {
		r.local.Set(key, true, r.redisTTL)
	}
	return banned, nil
}

func (r *BanCacheRepository) MarkBanned(userID int64, ttl time.Duration) {
	if r.local == nil {
		return
	}
	key := fmt.Sprintf("%d", userID)
	if ttl <= 0 {
		ttl = r.redisTTL
	}
	r.local.Set(key, true, ttl)
}

func (r *BanCacheRepository) UnmarkBanned(userID int64) {
	if r.local == nil {
		return
	}
	key := fmt.Sprintf("%d", userID)
	r.local.Delete(key)
}
