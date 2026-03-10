package statuscache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	primaryPrefix = "judge:status:"
	legacyPrefix  = "js:"
	// NullValue marks empty status cache entry.
	NullValue = "$NULL$"
)

// PrimaryKey returns the canonical cache key used by status/judge services.
func PrimaryKey(submissionID string) string {
	return primaryPrefix + submissionID
}

// LegacyKey returns the historical cache key used by submit/status writer.
func LegacyKey(submissionID string) string {
	return legacyPrefix + shortHash(submissionID)
}

// Get loads status payload from cache with primary->legacy fallback.
func Get(ctx context.Context, cacheClient *redis.Redis, submissionID string) (string, bool, error) {
	if cacheClient == nil || submissionID == "" {
		return "", false, nil
	}

	primary := PrimaryKey(submissionID)
	val, err := cacheClient.GetCtx(ctx, primary)
	if err != nil {
		if errors.Is(err, red.Nil) {
			val = ""
		} else {
			return "", false, err
		}
	}
	if val != "" {
		return val, true, nil
	}

	legacy := LegacyKey(submissionID)
	if legacy == primary {
		return "", false, nil
	}
	val, err = cacheClient.GetCtx(ctx, legacy)
	if err != nil {
		if errors.Is(err, red.Nil) {
			return "", false, nil
		}
		return "", false, err
	}
	if val == "" {
		return "", false, nil
	}
	return val, true, nil
}

// Set writes status payload to both primary and legacy keys.
func Set(ctx context.Context, cacheClient *redis.Redis, submissionID, value string, ttlSeconds int) error {
	if cacheClient == nil || submissionID == "" {
		return nil
	}
	keys := []string{PrimaryKey(submissionID), LegacyKey(submissionID)}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if ttlSeconds > 0 {
			if err := cacheClient.SetexCtx(ctx, key, value, ttlSeconds); err != nil {
				return err
			}
			continue
		}
		if err := cacheClient.SetCtx(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func shortHash(value string) string {
	if value == "" {
		return ""
	}
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:8])
}
