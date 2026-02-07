package repository

import (
	"context"
	"errors"
	"time"

	"fuzoj/internal/common/cache"
)

type BanCacheRepository interface {
	IsUserBanned(ctx context.Context, userID int64) (bool, error)
	MarkBanned(ctx context.Context, userID int64, ttl time.Duration) error
	UnmarkBanned(ctx context.Context, userID int64) error
}

type RedisBanCacheRepository struct {
	cache cache.Cache
}

func NewBanCacheRepository(cacheClient cache.Cache) BanCacheRepository {
	return &RedisBanCacheRepository{cache: cacheClient}
}

func (r *RedisBanCacheRepository) IsUserBanned(ctx context.Context, userID int64) (bool, error) {
	if r.cache == nil {
		return false, errors.New("cache is nil")
	}
	return r.cache.SIsMember(ctx, userBannedKey, userID)
}

func (r *RedisBanCacheRepository) MarkBanned(ctx context.Context, userID int64, ttl time.Duration) error {
	if r.cache == nil {
		return errors.New("cache is nil")
	}
	if err := r.cache.SAdd(ctx, userBannedKey, userID); err != nil {
		return err
	}
	return extendTTL(ctx, r.cache, userBannedKey, ttl)
}

func (r *RedisBanCacheRepository) UnmarkBanned(ctx context.Context, userID int64) error {
	if r.cache == nil {
		return errors.New("cache is nil")
	}
	return r.cache.SRem(ctx, userBannedKey, userID)
}
