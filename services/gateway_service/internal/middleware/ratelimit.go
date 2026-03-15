package middleware

import (
	"fmt"
	"net/http"
	"time"

	"fuzoj/services/gateway_service/internal/service"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RateLimitMiddleware enforces per-route rate limiting.
func RateLimitMiddleware(rateService *service.RateLimitService, defaultWindow time.Duration, globalRefillPerSec int, globalCapacity int) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if rateService == nil {
				next(w, r)
				return
			}
			if globalRefillPerSec > 0 && globalCapacity > 0 {
				if err := rateService.AllowTokenBucket(r.Context(), "gateway:rate:global", float64(globalRefillPerSec), float64(globalCapacity), 1); err != nil {
					WriteError(w, r, err)
					return
				}
			}

			policy := getRoutePolicy(r.Context())
			window := policy.RateLimit.Window
			if window == 0 {
				window = defaultWindow
			}

			clientIP := httpx.GetRemoteAddr(r)
			if policy.RateLimit.IPMax > 0 {
				key := fmt.Sprintf("gateway:rate:ip:%s:%s", clientIP, routeKey(policy))
				if err := rateService.Allow(r.Context(), key, policy.RateLimit.IPMax, window); err != nil {
					WriteError(w, r, err)
					return
				}
			}

			if policy.RateLimit.UserMax > 0 {
				if userID, ok := getUserID(r); ok {
					key := fmt.Sprintf("gateway:rate:user:%v:%s", userID, routeKey(policy))
					if err := rateService.Allow(r.Context(), key, policy.RateLimit.UserMax, window); err != nil {
						WriteError(w, r, err)
						return
					}
				}
			}

			if policy.RateLimit.RouteMax > 0 {
				key := fmt.Sprintf("gateway:rate:route:%s", routeKey(policy))
				if err := rateService.Allow(r.Context(), key, policy.RateLimit.RouteMax, window); err != nil {
					WriteError(w, r, err)
					return
				}
			}

			next(w, r)
		}
	}
}

func routeKey(policy RoutePolicy) string {
	if policy.Name != "" {
		return policy.Name
	}
	if policy.Path != "" {
		return policy.Path
	}
	return "unknown"
}
