package gateway_test

import (
	"context"
	"testing"
	"time"

	"fuzoj/services/gateway_service/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestBanCacheRepositoryLocalCache(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	defer redisClient.Close()

	local := repository.NewLRUCache(10, time.Minute)
	repo := repository.NewBanCacheRepository(local, redisClient, time.Minute)

	mini.SAdd("user:banned", "100")

	banned, err := repo.IsBanned(context.Background(), 100)
	if err != nil {
		t.Fatalf("is banned failed: %v", err)
	}
	if !banned {
		t.Fatalf("expected banned user")
	}

	if val, ok := local.Get("100"); !ok || !val {
		t.Fatalf("expected local cache to be populated")
	}
}

func TestBanCacheRepositoryMarkAndUnmark(t *testing.T) {
	local := repository.NewLRUCache(10, time.Minute)
	repo := repository.NewBanCacheRepository(local, nil, time.Minute)

	repo.MarkBanned(200, time.Minute)
	if val, ok := local.Get("200"); !ok || !val {
		t.Fatalf("expected banned mark to set local cache")
	}

	repo.UnmarkBanned(200)
	if _, ok := local.Get("200"); ok {
		t.Fatalf("expected unmark to clear local cache")
	}
}
