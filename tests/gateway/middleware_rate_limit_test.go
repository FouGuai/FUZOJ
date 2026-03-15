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
		middleware.RateLimitMiddleware(rateService, time.Second, 0, 0),
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
		middleware.RateLimitMiddleware(nil, time.Second, 10000, 50000),
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

func TestRateLimitServiceTokenBucketRefill(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	defer redisClient.Close()

	rateService := service.NewRateLimitService(redisClient, time.Second, time.Second)
	key := fmt.Sprintf("gateway:rate:route:%d", time.Now().UnixNano())

	if err := rateService.AllowTokenBucket(t.Context(), key, 2, 2, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rateService.AllowTokenBucket(t.Context(), key, 2, 2, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rateService.AllowTokenBucket(t.Context(), key, 2, 2, 1); err == nil {
		t.Fatalf("expected too many requests")
	}
	if ttl := mini.TTL(key); ttl <= 0 {
		t.Fatalf("expected ttl to be set")
	}
	mini.FastForward(500 * time.Millisecond)
	if err := rateService.AllowTokenBucket(t.Context(), key, 2, 2, 1); err == nil {
		t.Fatalf("expected still limited before full token refill")
	}
	mini.FastForward(500 * time.Millisecond)
	if err := rateService.AllowTokenBucket(t.Context(), key, 2, 2, 1); err != nil {
		t.Fatalf("expected token refill, got %v", err)
	}
}

func TestRateLimitMiddlewareGlobalTokenBucket(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}
	defer redisClient.Close()

	rateService := service.NewRateLimitService(redisClient, time.Second, time.Second)
	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/global", middleware.RoutePolicy{Path: "/global"})
	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	},
		middleware.RoutePolicyMiddleware(matcher),
		middleware.RateLimitMiddleware(rateService, time.Second, 2, 2),
	)
	for i := 0; i < 2; i++ {
		rec, _, reqErr := performRequest(http.HandlerFunc(handler), http.MethodGet, "/global", nil)
		if reqErr != nil {
			t.Fatalf("decode response failed: %v", reqErr)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status on attempt %d: %d", i+1, rec.Code)
		}
	}
	rec, _, reqErr := performRequest(http.HandlerFunc(handler), http.MethodGet, "/global", nil)
	if reqErr != nil {
		t.Fatalf("decode response failed: %v", reqErr)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}
