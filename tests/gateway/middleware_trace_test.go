package gateway_test

import (
	"net/http"
	"testing"

	"fuzoj/internal/gateway/middleware"

	"github.com/gin-gonic/gin"
)

func TestTraceMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.TraceMiddleware())
	router.GET("/trace", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec, _, err := performRequest(router, http.MethodGet, "/trace", nil)
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	traceID := rec.Header().Get("X-Trace-Id")
	if traceID == "" {
		t.Fatalf("expected trace id header")
	}
}

func TestTraceMiddlewarePreservesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.TraceMiddleware())
	router.GET("/trace", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec, _, err := performRequest(router, http.MethodGet, "/trace", map[string]string{"X-Request-Id": "req-123"})
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Header().Get("X-Request-Id") != "req-123" {
		t.Fatalf("expected request id header")
	}
}
