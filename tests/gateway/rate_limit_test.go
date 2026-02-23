package gateway_test

import (
	"context"
	"testing"
	"time"

	"fuzoj/pkg/errors"
	"fuzoj/services/gateway_service/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestRateLimitServiceAllow(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	defer redisClient.Close()

	service := service.NewRateLimitService(redisClient, time.Minute, time.Second)
	key := "gateway:rate:route:test"

	for i := 0; i < 2; i++ {
		if err := service.Allow(context.Background(), key, 2, time.Minute); err != nil {
			t.Fatalf("unexpected error on attempt %d: %v", i+1, err)
		}
	}

	err := service.Allow(context.Background(), key, 2, time.Minute)
	if err == nil || errors.GetCode(err) != errors.TooManyRequests {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}
