package gateway_test

import (
	"context"
	"testing"
	"time"

	"fuzoj/internal/gateway/repository"
)

func TestBanCacheRepositoryLocalCache(t *testing.T) {
	setCache := newMockSetCache()
	local := repository.NewLRUCache(10, time.Minute)
	repo := repository.NewBanCacheRepository(local, setCache, time.Second, time.Minute)

	if err := setCache.SAdd(context.Background(), "user:banned", int64(100)); err != nil {
		t.Fatalf("set cache add failed: %v", err)
	}

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
	repo := repository.NewBanCacheRepository(local, nil, time.Second, time.Minute)

	repo.MarkBanned(200, time.Minute)
	if val, ok := local.Get("200"); !ok || !val {
		t.Fatalf("expected banned mark to set local cache")
	}

	repo.UnmarkBanned(200)
	if _, ok := local.Get("200"); ok {
		t.Fatalf("expected unmark to clear local cache")
	}
}
