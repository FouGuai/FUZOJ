package gateway_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"fuzoj/internal/gateway/middleware"
	pkgerrors "fuzoj/pkg/errors"

	"github.com/gin-gonic/gin"
)

func TestProxyHandlerNilProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/proxy", middleware.ProxyHandler(nil, "route-a", 0, ""))

	rec, resp, err := performRequest(router, http.MethodGet, "/proxy", nil)
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

func TestProxyHandlerHeadersAndStripPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
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
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url failed: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("trace_id", "trace-1")
		c.Set("request_id", "req-1")
		c.Set("user_id", int64(42))
		c.Set("user_role", "admin")
		c.Next()
	})
	router.GET("/api/v1/echo", middleware.ProxyHandler(proxy, "route-a", 0, "/api/v1"))

	rec, _, err := performRequest(router, http.MethodGet, "/api/v1/echo", nil)
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if gotPath != "/echo" {
		t.Fatalf("unexpected upstream path: %s", gotPath)
	}
}

func TestProxyHandlerTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url failed: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	router := gin.New()
	router.GET("/slow", middleware.ProxyHandler(proxy, "route-slow", 10*time.Millisecond, ""))

	rec, _, err := performRequest(router, http.MethodGet, "/slow", nil)
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}
