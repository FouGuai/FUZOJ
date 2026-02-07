package cache

import (
	"context"
	"time"
)

// Cache defines the unified interface for cache operations.
// This abstraction allows switching between different cache implementations
// (Redis, Memcached, local memory) without changing business logic.
type Cache interface {
	BasicOps
	HashOps
	SetOps
	ZSetOps
	ListOps
	LockOps
	PipelineOps

	// Ping verifies the cache connection is alive
	Ping(ctx context.Context) error

	// Close closes the cache connection
	Close() error
}

// BasicOps defines basic key-value operations
type BasicOps interface {
	// Get retrieves the value for the given key
	Get(ctx context.Context, key string) (string, error)

	// Set stores a key-value pair with optional TTL
	// If ttl is 0, the key will not expire
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// SetNX sets the value only if the key does not exist (atomic operation)
	// Returns true if the key was set, false if it already existed
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)

	// GetSet atomically sets key to value and returns the old value
	GetSet(ctx context.Context, key string, value interface{}) (string, error)

	// Del deletes one or more keys
	Del(ctx context.Context, keys ...string) error

	// Exists checks if one or more keys exist
	// Returns the number of keys that exist
	Exists(ctx context.Context, keys ...string) (int64, error)

	// Expire sets a timeout on a key
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// TTL returns the remaining time to live of a key
	// Returns -1 if the key exists but has no expiration
	// Returns -2 if the key does not exist
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Incr increments the integer value of a key by 1
	Incr(ctx context.Context, key string) (int64, error)

	// IncrBy increments the integer value of a key by the given amount
	IncrBy(ctx context.Context, key string, value int64) (int64, error)

	// Decr decrements the integer value of a key by 1
	Decr(ctx context.Context, key string) (int64, error)

	// DecrBy decrements the integer value of a key by the given amount
	DecrBy(ctx context.Context, key string, value int64) (int64, error)
}

// HashOps defines hash (map) operations
type HashOps interface {
	// HSet sets field in the hash stored at key to value
	HSet(ctx context.Context, key, field string, value interface{}) error

	// HGet returns the value associated with field in the hash stored at key
	HGet(ctx context.Context, key, field string) (string, error)

	// HGetAll returns all fields and values of the hash stored at key
	HGetAll(ctx context.Context, key string) (map[string]string, error)

	// HMSet sets multiple fields in the hash stored at key
	HMSet(ctx context.Context, key string, fields map[string]interface{}) error

	// HMGet returns the values associated with the specified fields in the hash stored at key
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)

	// HDel deletes one or more fields from the hash stored at key
	HDel(ctx context.Context, key string, fields ...string) error

	// HExists checks if a field exists in the hash stored at key
	HExists(ctx context.Context, key, field string) (bool, error)

	// HLen returns the number of fields in the hash stored at key
	HLen(ctx context.Context, key string) (int64, error)

	// HIncrBy increments the integer value of a hash field by the given number
	HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error)
}

// SetOps defines set operations
type SetOps interface {
	// SAdd adds one or more members to a set
	SAdd(ctx context.Context, key string, members ...interface{}) error

	// SRem removes one or more members from a set
	SRem(ctx context.Context, key string, members ...interface{}) error

	// SMembers returns all members of a set
	SMembers(ctx context.Context, key string) ([]string, error)

	// SIsMember checks if a value is a member of a set
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)

	// SCard returns the number of members in a set
	SCard(ctx context.Context, key string) (int64, error)
}

// ZSetOps defines sorted set operations (crucial for leaderboard)
type ZSetOps interface {
	// ZAdd adds one or more members with scores to a sorted set
	ZAdd(ctx context.Context, key string, members ...ZMember) error

	// ZRem removes one or more members from a sorted set
	ZRem(ctx context.Context, key string, members ...string) error

	// ZScore returns the score of a member in a sorted set
	ZScore(ctx context.Context, key, member string) (float64, error)

	// ZIncrBy increments the score of a member in a sorted set
	ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error)

	// ZRange returns members in a sorted set by index range (ascending order)
	// start and stop are zero-based indexes
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// ZRangeWithScores returns members with scores in a sorted set by index range
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZMember, error)

	// ZRevRange returns members in a sorted set by index range (descending order)
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// ZRevRangeWithScores returns members with scores in descending order
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZMember, error)

	// ZRank returns the rank of a member in a sorted set (ascending order, 0-based)
	ZRank(ctx context.Context, key, member string) (int64, error)

	// ZRevRank returns the rank of a member in a sorted set (descending order, 0-based)
	ZRevRank(ctx context.Context, key, member string) (int64, error)

	// ZCard returns the number of members in a sorted set
	ZCard(ctx context.Context, key string) (int64, error)

	// ZRemRangeByRank removes members in a sorted set by index range
	ZRemRangeByRank(ctx context.Context, key string, start, stop int64) error
}

// ListOps defines list operations
type ListOps interface {
	// LPush prepends one or more values to a list
	LPush(ctx context.Context, key string, values ...interface{}) error

	// RPush appends one or more values to a list
	RPush(ctx context.Context, key string, values ...interface{}) error

	// LPop removes and returns the first element of a list
	LPop(ctx context.Context, key string) (string, error)

	// RPop removes and returns the last element of a list
	RPop(ctx context.Context, key string) (string, error)

	// LRange returns elements from a list by index range
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// LLen returns the length of a list
	LLen(ctx context.Context, key string) (int64, error)

	// LTrim trims a list to the specified range
	LTrim(ctx context.Context, key string, start, stop int64) error
}

// LockOps defines distributed lock operations
type LockOps interface {
	// TryLock attempts to acquire a distributed lock
	// Returns true if lock was acquired, false otherwise
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// Unlock releases a distributed lock
	Unlock(ctx context.Context, key string) error

	// ExtendLock extends the TTL of an existing lock
	ExtendLock(ctx context.Context, key string, ttl time.Duration) error
}

// PipelineOps defines pipeline operations for batching commands
type PipelineOps interface {
	// Pipeline executes multiple commands in a pipeline
	Pipeline(ctx context.Context, fn func(pipe Pipeliner) error) error
}

// Pipeliner defines the interface for pipeline operations
type Pipeliner interface {
	Set(key string, value interface{}, ttl time.Duration) error
	Get(key string) error
	Del(keys ...string) error
	Expire(key string, ttl time.Duration) error
	// Add more operations as needed
}

// ZMember represents a member in a sorted set with its score
type ZMember struct {
	Score  float64
	Member string
}

// ScanIterator defines the interface for scanning keys
type ScanIterator interface {
	Next(ctx context.Context) bool
	Val() string
	Err() error
}

// NullCacheValue is a sentinel value to represent null/empty data in cache
// This prevents cache penetration by caching the absence of data
const NullCacheValue = "$NULL$"

// GetWithCached implements cache-aside pattern with null value caching
// It tries to get data from cache first, if cache miss, it calls the fetch function
// and stores the result in cache. Empty results are also cached to prevent cache penetration.
//
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - cache: the Cache interface implementation
//   - key: the cache key to store/retrieve data
//   - ttl: time to live for the cached data
//   - emptyTTL: time to live for null/empty values (usually shorter than ttl)
//   - isEmpty: function to check if the result is empty/null
//   - marshal: function to serialize T to string
//   - unmarshal: function to deserialize string to T
//   - fn: function to fetch data from database/source if cache miss
//
// Example:
//
//	user, err := GetWithCached(ctx, cache, "user:123", 1*time.Hour, 5*time.Minute,
//		func(u *User) bool { return u == nil },
//		func(u *User) string { return json.Marshal(u) },
//		func(data string) (*User, error) { return json.Unmarshal(data) },
//		func(ctx context.Context) (*User, error) {
//			return db.GetUserByID(ctx, 123)
//		})
func GetWithCached[T any](
	ctx context.Context,
	cache Cache,
	key string,
	ttl time.Duration,
	emptyTTL time.Duration,
	isEmpty func(T) bool,
	marshal func(T) string,
	unmarshal func(string) (T, error),
	fn func(context.Context) (T, error),
) (T, error) {
	var zero T

	// Try to get from cache first
	if cached, err := cache.Get(ctx, key); err == nil && cached != "" {
		// Check if it's a null cached value
		if cached == NullCacheValue {
			return zero, nil
		}
		// Try to unmarshal from cache
		if result, err := unmarshal(cached); err == nil {
			return result, nil
		}
	}

	// Cache miss: fetch from database
	data, err := fn(ctx)
	if err != nil {
		return zero, err
	}

	// Cache empty values to prevent cache penetration
	if isEmpty(data) {
		_ = cache.Set(ctx, key, NullCacheValue, emptyTTL)
		return zero, nil
	}

	// Store in cache
	_ = cache.Set(ctx, key, marshal(data), ttl)
	return data, nil
}

// UpdateCached updates data and deletes the cache
// This implements write-through pattern by invalidating cache on write.
//
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - cache: the Cache interface implementation
//   - key: the cache key to invalidate
//   - fn: function to update data in database
//
// Example:
//
//	err := UpdateCached(ctx, cache, "user:123", func(ctx context.Context) error {
//		return db.UpdateUser(ctx, user)
//	})
func UpdateCached(
	ctx context.Context,
	cache Cache,
	key string,
	fn func(context.Context) error,
) error {
	// Execute the update
	if err := fn(ctx); err != nil {
		return err
	}

	// Delete the cache to force refresh on next read
	_ = cache.Del(ctx, key)
	return nil
}

// DeleteCached deletes data and clears the cache
// This implements write-through pattern by invalidating cache on delete.
//
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - cache: the Cache interface implementation
//   - key: the cache key to invalidate
//   - fn: function to delete data from database
//
// Example:
//
//	err := DeleteCached(ctx, cache, "user:123", func(ctx context.Context) error {
//		return db.DeleteUser(ctx, 123)
//	})
func DeleteCached(
	ctx context.Context,
	cache Cache,
	key string,
	fn func(context.Context) error,
) error {
	// Execute the delete
	if err := fn(ctx); err != nil {
		return err
	}

	// Delete the cache
	_ = cache.Del(ctx, key)
	return nil
}
