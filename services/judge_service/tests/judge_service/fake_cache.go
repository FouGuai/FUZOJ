package judge_service_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	cachex "fuzoj/internal/common/cache"
)

// fakeCache is a minimal in-memory cache for tests.
type fakeCache struct {
	mu   sync.Mutex
	data map[string]string
}

func newFakeCache() *fakeCache {
	return &fakeCache{
		data: make(map[string]string),
	}
}

func (f *fakeCache) Get(ctx context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data[key], nil
}

func (f *fakeCache) MGet(ctx context.Context, keys ...string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, f.data[key])
	}
	return out, nil
}

func (f *fakeCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := value.(string); ok {
		f.data[key] = s
		return nil
	}
	f.data[key] = fmt.Sprint(value)
	return nil
}

func (f *fakeCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.data[key]; ok {
		return false, nil
	}
	if s, ok := value.(string); ok {
		f.data[key] = s
		return true, nil
	}
	f.data[key] = fmt.Sprint(value)
	return true, nil
}

func (f *fakeCache) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	old := f.data[key]
	if s, ok := value.(string); ok {
		f.data[key] = s
	} else {
		f.data[key] = fmt.Sprint(value)
	}
	return old, nil
}

func (f *fakeCache) Del(ctx context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, key := range keys {
		delete(f.data, key)
	}
	return nil
}

func (f *fakeCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var count int64
	for _, key := range keys {
		if _, ok := f.data[key]; ok {
			count++
		}
	}
	return count, nil
}

func (f *fakeCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (f *fakeCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return -1, nil
}

func (f *fakeCache) Incr(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (f *fakeCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return 0, nil
}

func (f *fakeCache) Decr(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (f *fakeCache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return 0, nil
}

func (f *fakeCache) HSet(ctx context.Context, key, field string, value interface{}) error {
	return nil
}

func (f *fakeCache) HGet(ctx context.Context, key, field string) (string, error) {
	return "", nil
}

func (f *fakeCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return nil, nil
}

func (f *fakeCache) HMSet(ctx context.Context, key string, fields map[string]interface{}) error {
	return nil
}

func (f *fakeCache) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	return nil, nil
}

func (f *fakeCache) HDel(ctx context.Context, key string, fields ...string) error {
	return nil
}

func (f *fakeCache) HExists(ctx context.Context, key, field string) (bool, error) {
	return false, nil
}

func (f *fakeCache) HLen(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (f *fakeCache) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	return 0, nil
}

func (f *fakeCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return nil
}

func (f *fakeCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	return nil
}

func (f *fakeCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return nil, nil
}

func (f *fakeCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return false, nil
}

func (f *fakeCache) SCard(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (f *fakeCache) ZAdd(ctx context.Context, key string, members ...cachex.ZMember) error {
	return nil
}

func (f *fakeCache) ZRem(ctx context.Context, key string, members ...string) error {
	return nil
}

func (f *fakeCache) ZScore(ctx context.Context, key, member string) (float64, error) {
	return 0, nil
}

func (f *fakeCache) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	return 0, nil
}

func (f *fakeCache) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return nil, nil
}

func (f *fakeCache) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]cachex.ZMember, error) {
	return nil, nil
}

func (f *fakeCache) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return nil, nil
}

func (f *fakeCache) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]cachex.ZMember, error) {
	return nil, nil
}

func (f *fakeCache) ZRank(ctx context.Context, key, member string) (int64, error) {
	return 0, nil
}

func (f *fakeCache) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	return 0, nil
}

func (f *fakeCache) ZCard(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (f *fakeCache) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) error {
	return nil
}

func (f *fakeCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	return nil
}

func (f *fakeCache) RPush(ctx context.Context, key string, values ...interface{}) error {
	return nil
}

func (f *fakeCache) LPop(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (f *fakeCache) RPop(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (f *fakeCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return nil, nil
}

func (f *fakeCache) LLen(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (f *fakeCache) LTrim(ctx context.Context, key string, start, stop int64) error {
	return nil
}

func (f *fakeCache) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return true, nil
}

func (f *fakeCache) Unlock(ctx context.Context, key string) error {
	return nil
}

func (f *fakeCache) ExtendLock(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (f *fakeCache) Pipeline(ctx context.Context, fn func(pipe cachex.Pipeliner) error) error {
	return fn(fakePipeliner{})
}

func (f *fakeCache) Ping(ctx context.Context) error {
	return nil
}

func (f *fakeCache) Close() error {
	return nil
}

type fakePipeliner struct{}

func (fakePipeliner) Set(key string, value interface{}, ttl time.Duration) error {
	return nil
}

func (fakePipeliner) Get(key string) error {
	return nil
}

func (fakePipeliner) Del(keys ...string) error {
	return nil
}

func (fakePipeliner) Expire(key string, ttl time.Duration) error {
	return nil
}
