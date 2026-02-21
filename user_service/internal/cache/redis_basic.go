package cache

import (
	"context"
	"fmt"
	"time"

	commoncache "fuzoj/internal/common/cache"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

var _ commoncache.BasicOps = (*RedisBasicCache)(nil)

type RedisBasicCache struct {
	client *redis.Redis
}

func NewRedisBasicCache(client *redis.Redis) commoncache.BasicOps {
	if client == nil {
		return nil
	}
	return &RedisBasicCache{client: client}
}

func (r *RedisBasicCache) Get(ctx context.Context, key string) (string, error) {
	return r.client.GetCtx(ctx, key)
}

func (r *RedisBasicCache) MGet(ctx context.Context, keys ...string) ([]string, error) {
	return r.client.MgetCtx(ctx, keys...)
}

func (r *RedisBasicCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl > 0 {
		return r.client.SetexCtx(ctx, key, fmt.Sprint(value), int(ttl.Seconds()))
	}
	return r.client.SetCtx(ctx, key, fmt.Sprint(value))
}

func (r *RedisBasicCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if ttl > 0 {
		return r.client.SetnxExCtx(ctx, key, fmt.Sprint(value), int(ttl.Seconds()))
	}
	return r.client.SetnxCtx(ctx, key, fmt.Sprint(value))
}

func (r *RedisBasicCache) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	return r.client.GetSetCtx(ctx, key, fmt.Sprint(value))
}

func (r *RedisBasicCache) Del(ctx context.Context, keys ...string) error {
	_, err := r.client.DelCtx(ctx, keys...)
	return err
}

func (r *RedisBasicCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.ExistsManyCtx(ctx, keys...)
}

func (r *RedisBasicCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return r.client.ExpireCtx(ctx, key, int(ttl.Seconds()))
}

func (r *RedisBasicCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	seconds, err := r.client.TtlCtx(ctx, key)
	if err != nil {
		return 0, err
	}
	if seconds < 0 {
		return time.Duration(seconds), nil
	}
	return time.Duration(seconds) * time.Second, nil
}

func (r *RedisBasicCache) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.IncrCtx(ctx, key)
}

func (r *RedisBasicCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrbyCtx(ctx, key, value)
}

func (r *RedisBasicCache) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.DecrCtx(ctx, key)
}

func (r *RedisBasicCache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.DecrbyCtx(ctx, key, value)
}
