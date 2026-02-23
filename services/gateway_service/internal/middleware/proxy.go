package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fuzoj/pkg/utils/contextkey"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RouteMiddleware applies per-route timeout, prefix stripping, and header injection.
func RouteMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			policy := getRoutePolicy(r.Context())

			if policy.StripPrefix != "" && strings.HasPrefix(r.URL.Path, policy.StripPrefix) {
				path := strings.TrimPrefix(r.URL.Path, policy.StripPrefix)
				if path == "" {
					path = "/"
				}
				r.URL.Path = path
				r.URL.RawPath = path
			}

			injectHeaders(r, policy.Name)

			if policy.Timeout > 0 {
				ctx, cancel := context.WithTimeout(r.Context(), policy.Timeout)
				defer cancel()
				r = r.WithContext(ctx)
			}

			next(w, r)
		}
	}
}

func injectHeaders(r *http.Request, routeName string) {
	ctx := r.Context()
	if traceID, ok := ctx.Value(contextkey.TraceID).(string); ok {
		r.Header.Set("X-Trace-Id", traceID)
	}
	if requestID, ok := ctx.Value(contextkey.RequestID).(string); ok {
		r.Header.Set("X-Request-Id", requestID)
	}
	if userID, ok := getUserID(r); ok {
		r.Header.Set("X-User-Id", fmt.Sprintf("%d", userID))
	}
	if role, ok := getUserRole(ctx); ok {
		r.Header.Set("X-User-Role", role)
	}
	if routeName != "" {
		r.Header.Set("X-Route-Name", routeName)
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP == "" {
		r.Header.Set("X-Real-IP", httpx.GetRemoteAddr(r))
	}
}

func ensureTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
