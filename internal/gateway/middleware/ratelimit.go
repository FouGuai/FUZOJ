package middleware

import (
	"fmt"
	"time"

	"fuzoj/internal/gateway/service"
	"fuzoj/pkg/utils/response"

	"github.com/gin-gonic/gin"
)

type RateLimitPolicy struct {
	Window   time.Duration
	UserMax  int
	IPMax    int
	RouteMax int
}

// RateLimitMiddleware enforces per-route rate limiting.
func RateLimitMiddleware(rateService *service.RateLimitService, routeKey string, policy RateLimitPolicy, defaultWindow time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rateService == nil {
			c.Next()
			return
		}
		window := policy.Window
		if window == 0 {
			window = defaultWindow
		}
		clientIP := c.ClientIP()
		if policy.IPMax > 0 {
			key := fmt.Sprintf("gateway:rate:ip:%s:%s", clientIP, routeKey)
			if err := rateService.Allow(c.Request.Context(), key, policy.IPMax, window); err != nil {
				response.AbortWithError(c, err)
				return
			}
		}

		if policy.UserMax > 0 {
			if userID, ok := c.Get("user_id"); ok {
				key := fmt.Sprintf("gateway:rate:user:%v:%s", userID, routeKey)
				if err := rateService.Allow(c.Request.Context(), key, policy.UserMax, window); err != nil {
					response.AbortWithError(c, err)
					return
				}
			}
		}

		if policy.RouteMax > 0 {
			key := fmt.Sprintf("gateway:rate:route:%s", routeKey)
			if err := rateService.Allow(c.Request.Context(), key, policy.RouteMax, window); err != nil {
				response.AbortWithError(c, err)
				return
			}
		}

		c.Next()
	}
}
