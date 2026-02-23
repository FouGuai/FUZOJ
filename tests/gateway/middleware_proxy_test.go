package gateway_test

import (
	"net/http"
	"testing"
	"time"

	"fuzoj/services/gateway_service/internal/middleware"
	"fuzoj/services/gateway_service/internal/service"
)

func TestRouteMiddlewareHeadersAndStripPrefix(t *testing.T) {
	secret := "test-secret"
	issuer := "fuzoj"
	authService := service.NewAuthService(secret, issuer, nil, nil)

	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/api/v1/echo", middleware.RoutePolicy{
		Name:        "route-a",
		Path:        "/api/v1/echo",
		StripPrefix: "/api/v1",
		Auth:        middleware.AuthPolicy{Mode: "protected", Roles: []string{"admin"}},
		Timeout:     50 * time.Millisecond,
	})

	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/echo" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Trace-Id") == "" || r.Header.Get("X-Request-Id") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-User-Id") != "42" || r.Header.Get("X-User-Role") != "admin" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Route-Name") != "route-a" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	},
		middleware.TraceMiddleware(),
		middleware.RoutePolicyMiddleware(matcher),
		middleware.AuthMiddleware(authService),
		middleware.RouteMiddleware(),
	)

	token := newAccessToken(t, secret, issuer, 42, "admin")
	rec, _, err := performRequest(http.HandlerFunc(handler), http.MethodGet, "/api/v1/echo", map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}
