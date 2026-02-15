package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	traceIDHeader = "X-Trace-Id"
	requestIDHeader = "X-Request-Id"
)

// TraceMiddleware ensures each request has a trace ID for logs and responses.
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := strings.TrimSpace(c.GetHeader(traceIDHeader))
		if traceID == "" {
			traceID = uuid.NewString()
		}
		c.Set("trace_id", traceID)
		ctx := context.WithValue(c.Request.Context(), "trace_id", traceID)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set(traceIDHeader, traceID)

		requestID := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if requestID != "" {
			c.Set("request_id", requestID)
			ctx = context.WithValue(c.Request.Context(), "request_id", requestID)
			c.Request = c.Request.WithContext(ctx)
			c.Writer.Header().Set(requestIDHeader, requestID)
		}
		c.Next()
	}
}
