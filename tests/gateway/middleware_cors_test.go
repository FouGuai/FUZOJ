package gateway_test

import (
	"net/http"
	"testing"

	"fuzoj/internal/gateway/middleware"

	"github.com/gin-gonic/gin"
)

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name       string
		config     middleware.CORSConfig
		method     string
		origin     string
		wantStatus int
		wantHeader bool
	}{
		{
			name:       "disabled cors",
			config:     middleware.CORSConfig{Enabled: false},
			method:     http.MethodGet,
			origin:     "https://example.com",
			wantStatus: http.StatusOK,
		},
		{
			name: "allowed preflight",
			config: middleware.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Authorization"},
				MaxAge:         "600",
			},
			method:     http.MethodOptions,
			origin:     "https://example.com",
			wantStatus: http.StatusNoContent,
			wantHeader: true,
		},
		{
			name: "blocked preflight",
			config: middleware.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://allowed.com"},
			},
			method:     http.MethodOptions,
			origin:     "https://denied.com",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(middleware.CORSMiddleware(tc.config))
			router.GET("/resource", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			headers := map[string]string{}
			if tc.origin != "" {
				headers["Origin"] = tc.origin
			}
			rec, _, err := performRequest(router, tc.method, "/resource", headers)
			if err != nil {
				t.Fatalf("decode response failed: %v", err)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("unexpected status: %d", rec.Code)
			}
			if tc.wantHeader && rec.Header().Get("Access-Control-Allow-Origin") == "" {
				t.Fatalf("expected cors headers")
			}
		})
	}
}
