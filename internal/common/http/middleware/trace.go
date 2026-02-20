package middleware

import (
	"context"
	"strings"

	"fuzoj/pkg/utils/contextkey"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	traceIDHeader   = "X-Trace-Id"
	requestIDHeader = "X-Request-Id"
	userIDHeader    = "X-User-Id"

	traceIDContextKey   = "trace_id"
	requestIDContextKey = "request_id"
	userIDContextKey    = "user_id"
)

// TraceContextConfig controls how trace/request/user id are extracted and written.
type TraceContextConfig struct {
	AllowUserIDHeader bool
	WriteUserIDHeader bool
}

// TraceContextMiddleware ensures trace/request/user id are in context and response headers.
func TraceContextMiddleware() gin.HandlerFunc {
	return TraceContextMiddlewareWithConfig(TraceContextConfig{
		AllowUserIDHeader: true,
		WriteUserIDHeader: true,
	})
}

// TraceContextMiddlewareWithConfig is the configurable version of TraceContextMiddleware.
func TraceContextMiddlewareWithConfig(cfg TraceContextConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := strings.TrimSpace(c.GetHeader(traceIDHeader))
		if traceID == "" {
			traceID = uuid.NewString()
		}
		c.Set(traceIDContextKey, traceID)
		ctx := context.WithValue(c.Request.Context(), contextkey.TraceID, traceID)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set(traceIDHeader, traceID)

		requestID := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set(requestIDContextKey, requestID)
		ctx = context.WithValue(c.Request.Context(), contextkey.RequestID, requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set(requestIDHeader, requestID)

		if cfg.AllowUserIDHeader {
			userID := strings.TrimSpace(c.GetHeader(userIDHeader))
			if userID != "" {
				c.Set(userIDContextKey, userID)
				ctx = context.WithValue(c.Request.Context(), contextkey.UserID, userID)
				c.Request = c.Request.WithContext(ctx)
				if cfg.WriteUserIDHeader {
					c.Writer.Header().Set(userIDHeader, userID)
				}
			}
		}

		c.Next()
	}
}
