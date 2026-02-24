package gateway_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"fuzoj/services/gateway_service/internal/config"
	"fuzoj/services/gateway_service/internal/discovery"
	"fuzoj/services/gateway_service/internal/middleware"
	"fuzoj/services/gateway_service/internal/proxy"
	"fuzoj/services/gateway_service/internal/service"

	"github.com/zeromicro/go-zero/rest"
)

type upstreamCapture struct {
	Path    string
	Headers map[string]string
}

func TestGatewayForwardingBasic(t *testing.T) {
	secret := "test-secret"
	issuer := "fuzoj"
	captureCh := make(chan upstreamCapture, 1)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := map[string]string{
			"X-User-Id":    r.Header.Get("X-User-Id"),
			"X-User-Role":  r.Header.Get("X-User-Role"),
			"X-Route-Name": r.Header.Get("X-Route-Name"),
			"X-Trace-Id":   r.Header.Get("X-Trace-Id"),
			"X-Request-Id": r.Header.Get("X-Request-Id"),
			"X-Real-IP":    r.Header.Get("X-Real-IP"),
		}
		captureCh <- upstreamCapture{Path: r.URL.Path, Headers: headers}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer upstream.Close()

	port := pickFreePort(t)
	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/api/v1/echo", middleware.RoutePolicy{
		Name:        "route-echo",
		Path:        "/api/v1/echo",
		StripPrefix: "/api/v1",
		Auth:        middleware.AuthPolicy{Mode: "protected", Roles: []string{"admin"}},
	})

	authService := service.NewAuthService(secret, issuer, nil, nil)
	server := rest.MustNewServer(rest.RestConf{Host: "127.0.0.1", Port: port})
	defer server.Stop()
	server.Use(middleware.TraceMiddleware())
	server.Use(middleware.RoutePolicyMiddleware(matcher))
	server.Use(middleware.AuthMiddleware(authService))
	server.Use(middleware.RouteMiddleware())

	picker := discovery.NewRoundRobinPicker([]string{upstream.Listener.Addr().String()})
	handler := proxy.NewHTTPForwarder(picker, config.HttpClientConf{Prefix: "", Timeout: 3000})
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/echo", Handler: handler},
	})
	go server.Start()

	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/echo", port)
	if err := waitForGateway(url, 2*time.Second); err != nil {
		t.Fatalf("gateway not ready: %v", err)
	}

	token := newAccessToken(t, secret, issuer, 42, "admin", 5*time.Minute)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Request-Id", "req-123")
	req.Header.Set("X-Trace-Id", "trace-abc")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	select {
	case got := <-captureCh:
		if got.Path != "/echo" {
			t.Fatalf("unexpected upstream path: %s", got.Path)
		}
		if got.Headers["X-User-Id"] != "42" {
			t.Fatalf("unexpected X-User-Id: %s", got.Headers["X-User-Id"])
		}
		if got.Headers["X-User-Role"] != "admin" {
			t.Fatalf("unexpected X-User-Role: %s", got.Headers["X-User-Role"])
		}
		if got.Headers["X-Route-Name"] != "route-echo" {
			t.Fatalf("unexpected X-Route-Name: %s", got.Headers["X-Route-Name"])
		}
		if got.Headers["X-Request-Id"] != "req-123" {
			t.Fatalf("unexpected X-Request-Id: %s", got.Headers["X-Request-Id"])
		}
		if got.Headers["X-Trace-Id"] != "trace-abc" {
			t.Fatalf("unexpected X-Trace-Id: %s", got.Headers["X-Trace-Id"])
		}
		if got.Headers["X-Real-IP"] == "" {
			t.Fatalf("expected X-Real-IP")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("upstream did not receive request")
	}
}

func TestGatewayForwardingRoundRobin(t *testing.T) {
	secret := "test-secret"
	issuer := "fuzoj"

	var hitA int32
	var hitB int32

	upstreamA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hitA, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstreamA.Close()

	upstreamB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hitB, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstreamB.Close()

	port := pickFreePort(t)
	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/api/v1/ping", middleware.RoutePolicy{
		Name:        "route-ping",
		Path:        "/api/v1/ping",
		StripPrefix: "/api/v1",
		Auth:        middleware.AuthPolicy{Mode: "protected", Roles: []string{"admin"}},
	})

	authService := service.NewAuthService(secret, issuer, nil, nil)
	server := rest.MustNewServer(rest.RestConf{Host: "127.0.0.1", Port: port})
	defer server.Stop()
	server.Use(middleware.TraceMiddleware())
	server.Use(middleware.RoutePolicyMiddleware(matcher))
	server.Use(middleware.AuthMiddleware(authService))
	server.Use(middleware.RouteMiddleware())

	picker := discovery.NewRoundRobinPicker([]string{
		upstreamA.Listener.Addr().String(),
		upstreamB.Listener.Addr().String(),
	})
	handler := proxy.NewHTTPForwarder(picker, config.HttpClientConf{Prefix: "", Timeout: 3000})
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/ping", Handler: handler},
	})
	go server.Start()

	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/ping", port)
	if err := waitForGateway(url, 2*time.Second); err != nil {
		t.Fatalf("gateway not ready: %v", err)
	}

	token := newAccessToken(t, secret, issuer, 42, "admin", 5*time.Minute)
	for i := 0; i < 2; i++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("build request failed: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected status: %d", resp.StatusCode)
		}
	}

	if atomic.LoadInt32(&hitA) == 0 || atomic.LoadInt32(&hitB) == 0 {
		t.Fatalf("expected requests to hit both upstreams, got A=%d B=%d", hitA, hitB)
	}
}

func waitForGateway(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", url)
}
