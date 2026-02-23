package gateway_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/services/gateway_service/internal/middleware"
	"fuzoj/services/gateway_service/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestRateLimitMiddleware(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	defer redisClient.Close()

	rateService := service.NewRateLimitService(redisClient, time.Second, time.Second)
	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/limited", middleware.RoutePolicy{
		Name: "route-a",
		Path: "/limited",
		RateLimit: middleware.RateLimitPolicy{
			RouteMax: 2,
		},
	})

	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), contextkey.UserID, int64(7))
		r = r.WithContext(ctx)
		w.WriteHeader(http.StatusOK)
	},
		middleware.RoutePolicyMiddleware(matcher),
		middleware.RateLimitMiddleware(rateService, time.Second),
	)

	for i := 0; i < 2; i++ {
		rec, _, err := performRequest(http.HandlerFunc(handler), http.MethodGet, "/limited", map[string]string{"X-Forwarded-For": "192.0.2.1"})
		if err != nil {
			t.Fatalf("decode response failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status on attempt %d: %d", i+1, rec.Code)
		}
	}

	rec, resp, err := performRequest(http.HandlerFunc(handler), http.MethodGet, "/limited", map[string]string{"X-Forwarded-For": "192.0.2.1"})
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if resp.Code != int(errors.TooManyRequests) {
		t.Fatalf("unexpected error code: %d", resp.Code)
	}
}

func TestRateLimitMiddlewareNilService(t *testing.T) {
	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/open", middleware.RoutePolicy{Path: "/open"})

	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	},
		middleware.RoutePolicyMiddleware(matcher),
		middleware.RateLimitMiddleware(nil, time.Second),
	)

	rec, _, err := performRequest(http.HandlerFunc(handler), http.MethodGet, "/open", nil)
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestRateLimitServiceAllowCacheUnavailable(t *testing.T) {
	svc := service.NewRateLimitService(nil, time.Second, time.Second)
	err := svc.Allow(t.Context(), "gateway:rate:route:test", 1, time.Second)
	if err == nil {
		t.Fatalf("expected error")
	}
	if errors.GetCode(err) != errors.ServiceUnavailable {
		t.Fatalf("unexpected error code: %v", err)
	}
}

func TestRateLimitServiceExpireRefresh(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	defer redisClient.Close()

	rateService := service.NewRateLimitService(redisClient, time.Second, time.Second)
	key := fmt.Sprintf("gateway:rate:route:%d", time.Now().UnixNano())

	if err := rateService.Allow(t.Context(), key, 2, time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl := mini.TTL(key); ttl <= 0 {
		t.Fatalf("expected ttl to be set")
	}

	mini.Set(key, "1")
	mini.Persist(key)
	if err := rateService.Allow(t.Context(), key, 5, time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl := mini.TTL(key); ttl <= 0 {
		t.Fatalf("expected ttl refresh, got %v", ttl)
	}
}
