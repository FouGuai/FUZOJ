package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds the configuration for Redis client.
type RedisConfig struct {
	Addr            string
	Password        string
	DB              int
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolSize        int
	MinIdleConns    int
	PoolTimeout     time.Duration
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}

// DefaultRedisConfig returns a RedisConfig with sensible defaults.
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        20,
		MinIdleConns:    2,
		PoolTimeout:     4 * time.Second,
		ConnMaxIdleTime: 10 * time.Minute,
		ConnMaxLifetime: 30 * time.Minute,
	}
}

// RedisCache implements Cache using go-redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a Redis cache instance with default config.
func NewRedisCache(addr string) (*RedisCache, error) {
	config := DefaultRedisConfig()
	config.Addr = addr
	return NewRedisCacheWithConfig(config)
}

// NewRedisCacheWithConfig creates a Redis cache instance with custom config.
func NewRedisCacheWithConfig(config *RedisConfig) (*RedisCache, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Addr == "" {
		return nil, fmt.Errorf("addr cannot be empty")
	}

	options := &redis.Options{
		Addr:            config.Addr,
		Password:        config.Password,
		DB:              config.DB,
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: config.MinRetryBackoff,
		MaxRetryBackoff: config.MaxRetryBackoff,
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		PoolSize:        config.PoolSize,
		MinIdleConns:    config.MinIdleConns,
		PoolTimeout:     config.PoolTimeout,
		ConnMaxIdleTime: config.ConnMaxIdleTime,
		ConnMaxLifetime: config.ConnMaxLifetime,
	}

	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// NewRedisCacheWithClient creates a Redis cache from an existing redis.Client.
func NewRedisCacheWithClient(client *redis.Client) (*RedisCache, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	return &RedisCache{client: client}, nil
}

func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	value, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, ttl).Result()
}

func (r *RedisCache) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	oldValue, err := r.client.GetSet(ctx, key, value).Result()
	if err == redis.Nil {
		return "", nil
	}
	return oldValue, err
}

func (r *RedisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	return r.client.Exists(ctx, keys...).Result()
}

func (r *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

func (r *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *RedisCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, key, value).Result()
}

func (r *RedisCache) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

func (r *RedisCache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.DecrBy(ctx, key, value).Result()
}

func (r *RedisCache) HSet(ctx context.Context, key, field string, value interface{}) error {
	return r.client.HSet(ctx, key, field, value).Err()
}

func (r *RedisCache) HGet(ctx context.Context, key, field string) (string, error) {
	value, err := r.client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

func (r *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

func (r *RedisCache) HMSet(ctx context.Context, key string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return r.client.HSet(ctx, key, fields).Err()
}

func (r *RedisCache) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	if len(fields) == 0 {
		return []interface{}{}, nil
	}
	return r.client.HMGet(ctx, key, fields...).Result()
}

func (r *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	return r.client.HDel(ctx, key, fields...).Err()
}

func (r *RedisCache) HExists(ctx context.Context, key, field string) (bool, error) {
	return r.client.HExists(ctx, key, field).Result()
}

func (r *RedisCache) HLen(ctx context.Context, key string) (int64, error) {
	return r.client.HLen(ctx, key).Result()
}

func (r *RedisCache) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	return r.client.HIncrBy(ctx, key, field, incr).Result()
}

func (r *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	if len(members) == 0 {
		return nil
	}
	return r.client.SAdd(ctx, key, members...).Err()
}

func (r *RedisCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	if len(members) == 0 {
		return nil
	}
	return r.client.SRem(ctx, key, members...).Err()
}

func (r *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

func (r *RedisCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

func (r *RedisCache) SCard(ctx context.Context, key string) (int64, error) {
	return r.client.SCard(ctx, key).Result()
}

func (r *RedisCache) ZAdd(ctx context.Context, key string, members ...ZMember) error {
	if len(members) == 0 {
		return nil
	}
	redisMembers := make([]redis.Z, 0, len(members))
	for _, member := range members {
		redisMembers = append(redisMembers, redis.Z{Score: member.Score, Member: member.Member})
	}
	return r.client.ZAdd(ctx, key, redisMembers...).Err()
}

func (r *RedisCache) ZRem(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	items := make([]interface{}, 0, len(members))
	for _, member := range members {
		items = append(items, member)
	}
	return r.client.ZRem(ctx, key, items...).Err()
}

func (r *RedisCache) ZScore(ctx context.Context, key, member string) (float64, error) {
	score, err := r.client.ZScore(ctx, key, member).Result()
	if err == redis.Nil {
		return 0, nil
	}
	return score, err
}

func (r *RedisCache) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	return r.client.ZIncrBy(ctx, key, increment, member).Result()
}

func (r *RedisCache) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(ctx, key, start, stop).Result()
}

func (r *RedisCache) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZMember, error) {
	results, err := r.client.ZRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return nil, err
	}
	members := make([]ZMember, 0, len(results))
	for _, result := range results {
		member, ok := result.Member.(string)
		if !ok {
			member = fmt.Sprintf("%v", result.Member)
		}
		members = append(members, ZMember{Score: result.Score, Member: member})
	}
	return members, nil
}

func (r *RedisCache) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRevRange(ctx, key, start, stop).Result()
}

func (r *RedisCache) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZMember, error) {
	results, err := r.client.ZRevRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return nil, err
	}
	members := make([]ZMember, 0, len(results))
	for _, result := range results {
		member, ok := result.Member.(string)
		if !ok {
			member = fmt.Sprintf("%v", result.Member)
		}
		members = append(members, ZMember{Score: result.Score, Member: member})
	}
	return members, nil
}

func (r *RedisCache) ZRank(ctx context.Context, key, member string) (int64, error) {
	rank, err := r.client.ZRank(ctx, key, member).Result()
	if err == redis.Nil {
		return -1, nil
	}
	return rank, err
}

func (r *RedisCache) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	rank, err := r.client.ZRevRank(ctx, key, member).Result()
	if err == redis.Nil {
		return -1, nil
	}
	return rank, err
}

func (r *RedisCache) ZCard(ctx context.Context, key string) (int64, error) {
	return r.client.ZCard(ctx, key).Result()
}

func (r *RedisCache) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) error {
	return r.client.ZRemRangeByRank(ctx, key, start, stop).Err()
}

func (r *RedisCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	if len(values) == 0 {
		return nil
	}
	return r.client.LPush(ctx, key, values...).Err()
}

func (r *RedisCache) RPush(ctx context.Context, key string, values ...interface{}) error {
	if len(values) == 0 {
		return nil
	}
	return r.client.RPush(ctx, key, values...).Err()
}

func (r *RedisCache) LPop(ctx context.Context, key string) (string, error) {
	value, err := r.client.LPop(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

func (r *RedisCache) RPop(ctx context.Context, key string) (string, error) {
	value, err := r.client.RPop(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

func (r *RedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, key, start, stop).Result()
}

func (r *RedisCache) LLen(ctx context.Context, key string) (int64, error) {
	return r.client.LLen(ctx, key).Result()
}

func (r *RedisCache) LTrim(ctx context.Context, key string, start, stop int64) error {
	return r.client.LTrim(ctx, key, start, stop).Err()
}

func (r *RedisCache) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, "1", ttl).Result()
}

func (r *RedisCache) Unlock(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCache) ExtendLock(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

func (r *RedisCache) Pipeline(ctx context.Context, fn func(pipe Pipeliner) error) error {
	if fn == nil {
		return nil
	}
	pipe := r.client.Pipeline()
	wrapper := &redisPipeliner{ctx: ctx, pipe: pipe}
	if err := fn(wrapper); err != nil {
		return err
	}
	_, err := pipe.Exec(ctx)
	return err
}

type redisPipeliner struct {
	ctx  context.Context
	pipe redis.Pipeliner
}

func (p *redisPipeliner) Set(key string, value interface{}, ttl time.Duration) error {
	return p.pipe.Set(p.ctx, key, value, ttl).Err()
}

func (p *redisPipeliner) Get(key string) error {
	return p.pipe.Get(p.ctx, key).Err()
}

func (p *redisPipeliner) Del(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return p.pipe.Del(p.ctx, keys...).Err()
}

func (p *redisPipeliner) Expire(key string, ttl time.Duration) error {
	return p.pipe.Expire(p.ctx, key, ttl).Err()
}

var _ Cache = (*RedisCache)(nil)
