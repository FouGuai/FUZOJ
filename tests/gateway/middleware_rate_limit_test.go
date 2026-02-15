package gateway_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"fuzoj/internal/gateway/middleware"
	"fuzoj/internal/gateway/service"
	pkgerrors "fuzoj/pkg/errors"

	"github.com/gin-gonic/gin"
)

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := newMockBasicCache()
	rateService := service.NewRateLimitService(cache, time.Second, time.Second)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(7))
		c.Next()
	})
	router.Use(middleware.RateLimitMiddleware(rateService, "route-a", middleware.RateLimitPolicy{
		RouteMax: 2,
	}, time.Second))
	router.GET("/limited", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		rec, _, err := performRequest(router, http.MethodGet, "/limited", map[string]string{"X-Forwarded-For": "192.0.2.1"})
		if err != nil {
			t.Fatalf("decode response failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status on attempt %d: %d", i+1, rec.Code)
		}
	}

	rec, resp, err := performRequest(router, http.MethodGet, "/limited", map[string]string{"X-Forwarded-For": "192.0.2.1"})
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if resp.Code != int(pkgerrors.TooManyRequests) {
		t.Fatalf("unexpected error code: %d", resp.Code)
	}
}

func TestRateLimitMiddlewareNilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RateLimitMiddleware(nil, "route-a", middleware.RateLimitPolicy{RouteMax: 1}, time.Second))
	router.GET("/open", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec, _, err := performRequest(router, http.MethodGet, "/open", nil)
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestRateLimitServiceAllowCacheUnavailable(t *testing.T) {
	service := service.NewRateLimitService(nil, time.Second, time.Second)
	err := service.Allow(t.Context(), "gateway:rate:route:test", 1, time.Second)
	if err == nil {
		t.Fatalf("expected error")
	}
	if pkgerrors.GetCode(err) != pkgerrors.ServiceUnavailable {
		t.Fatalf("unexpected error code: %v", err)
	}
}

func TestRateLimitServiceExpireRefresh(t *testing.T) {
	cache := newMockBasicCache()
	rateService := service.NewRateLimitService(cache, time.Second, time.Second)
	key := fmt.Sprintf("gateway:rate:route:%d", time.Now().UnixNano())

	if err := rateService.Allow(t.Context(), key, 2, time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cache.expires[key]; !ok {
		t.Fatalf("expected ttl to be set")
	}

	cache.mu.Lock()
	cache.values[key] = 1
	delete(cache.expires, key)
	cache.mu.Unlock()
	if err := rateService.Allow(t.Context(), key, 5, time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl, _ := cache.TTL(t.Context(), key); ttl <= 0 {
		t.Fatalf("expected ttl refresh, got %v", ttl)
	}
}
