package middleware

import (
	"context"
	"net/http"
	"strings"

	"fuzoj/pkg/utils/contextkey"

	"github.com/google/uuid"
)

const (
	traceIDHeader   = "X-Trace-Id"
	requestIDHeader = "X-Request-Id"
	userIDHeader    = "X-User-Id"
)

// TraceMiddleware ensures each request has a trace ID for logs and responses.
func TraceMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del(userIDHeader)

			traceID := strings.TrimSpace(r.Header.Get(traceIDHeader))
			if traceID == "" {
				traceID = uuid.NewString()
			}
			ctx := context.WithValue(r.Context(), contextkey.TraceID, traceID)
			w.Header().Set(traceIDHeader, traceID)

			requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
			if requestID == "" {
				requestID = uuid.NewString()
			}
			ctx = context.WithValue(ctx, contextkey.RequestID, requestID)
			w.Header().Set(requestIDHeader, requestID)

			r = r.WithContext(ctx)
			next(w, r)
		}
	}
}
