package gateway_test

import (
	"context"
	"testing"
	"time"

	"fuzoj/services/gateway_service/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestTokenBlacklistRepositoryLocalCache(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	defer redisClient.Close()

	local := repository.NewLRUCache(10, time.Minute)
	repo := repository.NewTokenBlacklistRepository(local, redisClient, time.Minute)

	mini.SAdd("token:blacklist", "hash-1")

	blacklisted, err := repo.IsBlacklisted(context.Background(), "hash-1")
	if err != nil {
		t.Fatalf("is blacklisted failed: %v", err)
	}
	if !blacklisted {
		t.Fatalf("expected blacklisted token")
	}

	if val, ok := local.Get("hash-1"); !ok || !val {
		t.Fatalf("expected local cache to be populated")
	}
}

func TestTokenBlacklistRepositoryEmptyHash(t *testing.T) {
	local := repository.NewLRUCache(10, time.Minute)
	repo := repository.NewTokenBlacklistRepository(local, nil, time.Minute)

	blacklisted, err := repo.IsBlacklisted(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blacklisted {
		t.Fatalf("expected empty hash to be not blacklisted")
	}
}
