package gateway_test

import (
	"context"
	"testing"
	"time"

	"fuzoj/internal/gateway/repository"
)

func TestTokenBlacklistRepositoryLocalCache(t *testing.T) {
	setCache := newMockSetCache()
	local := repository.NewLRUCache(10, time.Minute)
	repo := repository.NewTokenBlacklistRepository(local, setCache, time.Second, time.Minute)

	if err := setCache.SAdd(context.Background(), "token:blacklist", "hash-1"); err != nil {
		t.Fatalf("set cache add failed: %v", err)
	}

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
	repo := repository.NewTokenBlacklistRepository(local, nil, time.Second, time.Minute)

	blacklisted, err := repo.IsBlacklisted(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blacklisted {
		t.Fatalf("expected empty hash to be not blacklisted")
	}
}
