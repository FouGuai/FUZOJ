package cache

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"
)

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

func JitterTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	maxJitter := int64(ttl / 10)
	if maxJitter <= 0 {
		return ttl
	}
	n, err := rand.Int(rand.Reader, big.NewInt(maxJitter+1))
	if err != nil {
		return ttl
	}
	return ttl - time.Duration(n.Int64())
}
