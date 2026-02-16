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
	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	issuer := "fuzoj"
	authService := service.NewAuthService(secret, issuer, nil, nil)

	router := gin.New()
	router.GET("/protected", middleware.AuthMiddleware(authService, middleware.AuthPolicy{Mode: "protected", Roles: []string{"admin"}}), func(c *gin.Context) {
		if userID, ok := c.Get("user_id"); ok {
			c.Header("X-User-Id", fmt.Sprint(userID))
		}
		c.Status(http.StatusOK)
	})

	router.GET("/public", middleware.AuthMiddleware(authService, middleware.AuthPolicy{Mode: "public"}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	token := newAccessToken(t, secret, issuer, 42, "admin")

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
			wantCode:   int(pkgerrors.TokenInvalid),
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
			rec, resp, err := performRequest(router, http.MethodGet, tc.path, headers)
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
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	issuer := "fuzoj"
	authService := service.NewAuthService(secret, issuer, nil, nil)

	router := gin.New()
	router.GET("/protected", middleware.AuthMiddleware(authService, middleware.AuthPolicy{Mode: "protected", Roles: []string{"admin"}}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	token := newAccessToken(t, secret, issuer, 99, "user")
	rec, resp, err := performRequest(router, http.MethodGet, "/protected", map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if resp.Code != int(pkgerrors.Forbidden) {
		t.Fatalf("unexpected error code: %d", resp.Code)
	}
}

func TestAuthMiddlewareNilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.AuthMiddleware(nil, middleware.AuthPolicy{Mode: "protected"}))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec, resp, err := performRequest(router, http.MethodGet, "/protected", nil)
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if resp.Code != int(pkgerrors.ServiceUnavailable) {
		t.Fatalf("unexpected error code: %d", resp.Code)
	}
}

func TestAuthServiceAuthenticateErrors(t *testing.T) {
	secret := "test-secret"
	issuer := "fuzoj"
	authService := service.NewAuthService(secret, issuer, nil, nil)

	expiredToken := newTokenWithClaims(t, secret, issuer, 1, "user", time.Now().Add(-1*time.Minute))
	wrongIssuerToken := newTokenWithClaims(t, secret, "other", 1, "user", time.Now().Add(1*time.Minute))
	wrongTypeToken := replaceTokenType(t, secret, issuer)

	cases := []struct {
		name     string
		token    string
		wantCode pkgerrors.ErrorCode
	}{
		{name: "empty token", token: "", wantCode: pkgerrors.TokenInvalid},
		{name: "expired token", token: expiredToken, wantCode: pkgerrors.TokenExpired},
		{name: "wrong issuer", token: wrongIssuerToken, wantCode: pkgerrors.TokenInvalid},
		{name: "wrong type", token: wrongTypeToken, wantCode: pkgerrors.TokenInvalid},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := authService.Authenticate(t.Context(), tc.token)
			if err == nil {
				t.Fatalf("expected error")
			}
			if pkgerrors.GetCode(err) != tc.wantCode {
				t.Fatalf("unexpected error code: %v", err)
			}
		})
	}
}

func newTokenWithClaims(t *testing.T, secret, issuer string, userID int64, role string, exp time.Time) string {
	t.Helper()
	claims := map[string]interface{}{
		"role": role,
		"typ":  "access",
		"sub":  fmt.Sprintf("%d", userID),
		"iss":  issuer,
		"iat":  time.Now().Unix(),
		"exp":  exp.Unix(),
	}
	return signTokenWithClaims(t, secret, claims)
}

func replaceTokenType(t *testing.T, secret, issuer string) string {
	t.Helper()
	claims := map[string]interface{}{
		"role": "user",
		"typ":  "refresh",
		"sub":  "1",
		"iss":  issuer,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Minute).Unix(),
	}
	return signTokenWithClaims(t, secret, claims)
}

func signTokenWithClaims(t *testing.T, secret string, claims map[string]interface{}) string {
	t.Helper()
	token := newJWTToken(claims)
	raw, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}
	return raw
}

func newJWTToken(claims map[string]interface{}) *jwt.Token {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims))
}
