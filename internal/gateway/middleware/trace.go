package middleware

import (
	commonmw "fuzoj/internal/common/http/middleware"

	"github.com/gin-gonic/gin"
)

// TraceMiddleware ensures each request has a trace ID for logs and responses.
func TraceMiddleware() gin.HandlerFunc {
	mw := commonmw.TraceContextMiddlewareWithConfig(commonmw.TraceContextConfig{
		AllowUserIDHeader: false,
		WriteUserIDHeader: false,
	})
	return func(c *gin.Context) {
		c.Request.Header.Del("X-User-Id")
		mw(c)
	}
}
