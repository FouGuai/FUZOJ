package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	pkgerrors "fuzoj/pkg/errors"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

var tokenBucketScript = redis.NewScript(`
local key = KEYS[1]
local nowMs = tonumber(ARGV[1]) or 0
local refillPerSec = tonumber(ARGV[2]) or 0
local capacity = tonumber(ARGV[3]) or 0
local cost = tonumber(ARGV[4]) or 1
local ttlMs = tonumber(ARGV[5]) or 1000

if refillPerSec <= 0 or capacity <= 0 or cost <= 0 then
	return {0, -1}
end

local data = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(data[1])
local ts = tonumber(data[2])
if tokens == nil then
	tokens = capacity
end
if ts == nil then
	ts = nowMs
end

local elapsed = nowMs - ts
if elapsed < 0 then
	elapsed = 0
end

tokens = math.min(capacity, tokens + (elapsed * refillPerSec / 1000.0))
local allowed = 0
if tokens >= cost then
	allowed = 1
	tokens = tokens - cost
end

redis.call("HSET", key, "tokens", tostring(tokens), "ts", tostring(nowMs))
redis.call("PEXPIRE", key, ttlMs)
return {allowed, tokens}
`)

// RateLimitService enforces token-bucket limits using Redis.
type RateLimitService struct {
	cache        *redis.Redis
	window       time.Duration
	redisTimeout time.Duration
}

func NewRateLimitService(cacheClient *redis.Redis, window time.Duration, redisTimeout time.Duration) *RateLimitService {
	return &RateLimitService{cache: cacheClient, window: window, redisTimeout: redisTimeout}
}

func (s *RateLimitService) Allow(ctx context.Context, key string, max int, window time.Duration) error {
	if max <= 0 {
		return nil
	}
	if window <= 0 {
		window = s.window
	}
	if window <= 0 {
		window = time.Second
	}
	refillPerSec := float64(max) / window.Seconds()
	return s.AllowTokenBucket(ctx, key, refillPerSec, float64(max), 1)
}

func (s *RateLimitService) AllowTokenBucket(ctx context.Context, key string, refillPerSec, capacity, cost float64) error {
	if s.cache == nil {
		return pkgerrors.New(pkgerrors.ServiceUnavailable).WithMessage("rate limit cache is unavailable")
	}
	if refillPerSec <= 0 || capacity <= 0 || cost <= 0 {
		return nil
	}
	ctxCache, cancel := context.WithTimeout(ctx, s.redisTimeout)
	defer cancel()
	nowMs := time.Now().UnixMilli()
	ttlMs := tokenBucketTTLMillis(refillPerSec, capacity)
	result, err := s.cache.ScriptRunCtx(
		ctxCache,
		tokenBucketScript,
		[]string{key},
		nowMs,
		strconv.FormatFloat(refillPerSec, 'f', 6, 64),
		strconv.FormatFloat(capacity, 'f', 6, 64),
		strconv.FormatFloat(cost, 'f', 6, 64),
		ttlMs,
	)
	if err != nil {
		return pkgerrors.Wrapf(err, pkgerrors.CacheError, "rate limit check failed")
	}
	allowed, parseErr := parseTokenBucketAllowed(result)
	if parseErr != nil {
		return pkgerrors.Wrapf(parseErr, pkgerrors.CacheError, "rate limit script result parse failed")
	}
	if !allowed {
		return pkgerrors.New(pkgerrors.TooManyRequests).WithMessage(fmt.Sprintf("rate limit exceeded for %s", key))
	}
	return nil
}

func tokenBucketTTLMillis(refillPerSec, capacity float64) int64 {
	if refillPerSec <= 0 || capacity <= 0 {
		return int64(time.Second / time.Millisecond)
	}
	seconds := int64(math.Ceil(capacity / refillPerSec))
	if seconds < 1 {
		seconds = 1
	}
	// Keep idle keys for at least two refill cycles to limit stale-key buildup.
	return (seconds * 2) * int64(time.Second/time.Millisecond)
}

func parseTokenBucketAllowed(result any) (bool, error) {
	values, ok := result.([]any)
	if !ok || len(values) < 1 {
		return false, fmt.Errorf("unexpected script result type: %T", result)
	}
	switch v := values[0].(type) {
	case int64:
		return v == 1, nil
	case int:
		return v == 1, nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return false, err
		}
		return parsed == 1, nil
	case []byte:
		parsed, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return false, err
		}
		return parsed == 1, nil
	default:
		return false, fmt.Errorf("unexpected allowed type: %T", values[0])
	}
}
