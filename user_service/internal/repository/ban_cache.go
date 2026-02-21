package repository

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type BanCacheRepository interface {
	IsUserBanned(ctx context.Context, userID int64) (bool, error)
	MarkBanned(ctx context.Context, userID int64, ttl time.Duration) error
	UnmarkBanned(ctx context.Context, userID int64) error
}

type RedisBanCacheRepository struct {
	redis *redis.Redis
}

func NewBanCacheRepository(redisClient *redis.Redis) BanCacheRepository {
	return &RedisBanCacheRepository{redis: redisClient}
}

func (r *RedisBanCacheRepository) IsUserBanned(ctx context.Context, userID int64) (bool, error) {
	if r.redis == nil {
		return false, errors.New("cache is nil")
	}
	return r.redis.SismemberCtx(ctx, userBannedKey, userID)
}

func (r *RedisBanCacheRepository) MarkBanned(ctx context.Context, userID int64, ttl time.Duration) error {
	if r.redis == nil {
		return errors.New("cache is nil")
	}
	if _, err := r.redis.SaddCtx(ctx, userBannedKey, userID); err != nil {
		return err
	}
	return extendRedisTTL(ctx, r.redis, userBannedKey, ttl)
}

func (r *RedisBanCacheRepository) UnmarkBanned(ctx context.Context, userID int64) error {
	if r.redis == nil {
		return errors.New("cache is nil")
	}
	_, err := r.redis.SremCtx(ctx, userBannedKey, userID)
	return err
}
