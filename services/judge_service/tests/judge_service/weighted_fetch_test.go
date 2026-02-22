package judge_service_test

import (
	"context"
	"testing"
	"time"

	"fuzoj/internal/common/mq"
)

func TestTokenLimiter(t *testing.T) {
	limiter := mq.NewTokenLimiter(1)
	if err := limiter.Acquire(context.Background()); err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := limiter.Acquire(ctx); err == nil {
		t.Fatal("expected acquire to block")
	}
	limiter.Release()
	if err := limiter.Acquire(context.Background()); err != nil {
		t.Fatalf("acquire after release failed: %v", err)
	}
}
