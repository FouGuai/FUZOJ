package gateway_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/services/gateway_service/internal/middleware"
	"fuzoj/services/gateway_service/internal/service"
)

func TestAuthMiddlewareBasic(t *testing.T) {
	secret := "test-secret"
	issuer := "fuzoj"
	authService := service.NewAuthService(secret, issuer, nil, nil)
	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/protected", middleware.RoutePolicy{Auth: middleware.AuthPolicy{Mode: "protected", Roles: []string{"admin"}}, Path: "/protected"})
	matcher.AddExact(http.MethodGet, "/public", middleware.RoutePolicy{Auth: middleware.AuthPolicy{Mode: "public"}, Path: "/public"})

	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if userID, ok := r.Context().Value(contextkey.UserID).(int64); ok {
			w.Header().Set("X-User-Id", fmt.Sprint(userID))
		}
		w.WriteHeader(http.StatusOK)
	},
		middleware.RoutePolicyMiddleware(matcher),
		middleware.AuthMiddleware(authService),
	)

	token := newAccessToken(t, secret, issuer, 42, "admin", 5*time.Minute)

	cases := []struct {
		name       string
		path       string
		authHeader string
		wantStatus int
		wantCode   int
		wantUserID string
	}{
		{
			name:       "public without token",
			path:       "/public",
			wantStatus: http.StatusOK,
		},
		{
			name:       "protected missing token",
			path:       "/protected",
			wantStatus: http.StatusUnauthorized,
			wantCode:   int(errors.TokenInvalid),
		},
		{
			name:       "protected valid token",
			path:       "/protected",
			authHeader: "Bearer " + token,
			wantStatus: http.StatusOK,
			wantUserID: "42",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if tc.authHeader != "" {
				headers["Authorization"] = tc.authHeader
			}
			rec, resp, err := performRequest(http.HandlerFunc(handler), http.MethodGet, tc.path, headers)
			if err != nil {
				t.Fatalf("decode response failed: %v", err)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("unexpected status: %d", rec.Code)
			}
			if tc.wantCode != 0 && resp.Code != tc.wantCode {
				t.Fatalf("unexpected error code: %d", resp.Code)
			}
			if tc.wantUserID != "" && rec.Header().Get("X-User-Id") != tc.wantUserID {
				t.Fatalf("unexpected user id header: %s", rec.Header().Get("X-User-Id"))
			}
		})
	}
}

func TestAuthMiddlewareRoleDenied(t *testing.T) {
	secret := "test-secret"
	issuer := "fuzoj"
	authService := service.NewAuthService(secret, issuer, nil, nil)
	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/protected", middleware.RoutePolicy{Auth: middleware.AuthPolicy{Mode: "protected", Roles: []string{"admin"}}, Path: "/protected"})

	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	},
		middleware.RoutePolicyMiddleware(matcher),
		middleware.AuthMiddleware(authService),
	)

	token := newAccessToken(t, secret, issuer, 99, "user", 5*time.Minute)
	rec, resp, err := performRequest(http.HandlerFunc(handler), http.MethodGet, "/protected", map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if resp.Code != int(errors.Forbidden) {
		t.Fatalf("unexpected error code: %d", resp.Code)
	}
}

func TestAuthMiddlewareNilService(t *testing.T) {
	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/protected", middleware.RoutePolicy{Auth: middleware.AuthPolicy{Mode: "protected"}, Path: "/protected"})

	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	},
		middleware.RoutePolicyMiddleware(matcher),
		middleware.AuthMiddleware(nil),
	)

	rec, resp, err := performRequest(http.HandlerFunc(handler), http.MethodGet, "/protected", nil)
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if resp.Code != int(errors.ServiceUnavailable) {
		t.Fatalf("unexpected error code: %d", resp.Code)
	}
}
