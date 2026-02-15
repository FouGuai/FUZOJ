package gateway_test

import (
	"testing"
	"time"

	"fuzoj/internal/gateway/repository"
)

func TestLRUCacheSetGetAndExpire(t *testing.T) {
	cache := repository.NewLRUCache(2, 10*time.Millisecond)
	cache.Set("a", true, 0)

	if val, ok := cache.Get("a"); !ok || !val {
		t.Fatalf("expected cached value")
	}

	time.Sleep(15 * time.Millisecond)
	if _, ok := cache.Get("a"); ok {
		t.Fatalf("expected value to expire")
	}
}

func TestLRUCacheEviction(t *testing.T) {
	cache := repository.NewLRUCache(2, time.Minute)
	cache.Set("a", true, 0)
	cache.Set("b", true, 0)
	cache.Get("a")
	cache.Set("c", true, 0)

	if _, ok := cache.Get("b"); ok {
		t.Fatalf("expected oldest entry to be evicted")
	}
	if _, ok := cache.Get("a"); !ok {
		t.Fatalf("expected recent entry to remain")
	}
	if _, ok := cache.Get("c"); !ok {
		t.Fatalf("expected newest entry to remain")
	}
}
