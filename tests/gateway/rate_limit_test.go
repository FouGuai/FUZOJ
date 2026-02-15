package gateway_test

import (
	"context"
	"testing"
	"time"

	"fuzoj/internal/gateway/service"
	pkgerrors "fuzoj/pkg/errors"
)

func TestRateLimitServiceAllow(t *testing.T) {
	cache := newMockBasicCache()
	service := service.NewRateLimitService(cache, time.Minute, time.Second)
	key := "gateway:rate:route:test"

	for i := 0; i < 2; i++ {
		if err := service.Allow(context.Background(), key, 2, time.Minute); err != nil {
			t.Fatalf("unexpected error on attempt %d: %v", i+1, err)
		}
	}

	err := service.Allow(context.Background(), key, 2, time.Minute)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.TooManyRequests {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}
