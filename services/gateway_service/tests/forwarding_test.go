package gateway_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fuzoj/services/gateway_service/internal/middleware"
	"fuzoj/services/gateway_service/internal/service"

	"github.com/zeromicro/go-zero/gateway"
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
	gwConf := gateway.GatewayConf{
		RestConf: rest.RestConf{Host: "127.0.0.1", Port: port},
		Upstreams: []gateway.Upstream{
			{
				Name: "upstream-a",
				Http: &gateway.HttpClientConf{Target: upstream.Listener.Addr().String(), Prefix: "", Timeout: 3000},
				Mappings: []gateway.RouteMapping{
					{Method: http.MethodGet, Path: "/api/v1/echo"},
				},
			},
		},
	}

	matcher := middleware.NewPolicyMatcher()
	matcher.AddExact(http.MethodGet, "/api/v1/echo", middleware.RoutePolicy{
		Name:        "route-echo",
		Path:        "/api/v1/echo",
		StripPrefix: "/api/v1",
		Auth:        middleware.AuthPolicy{Mode: "protected", Roles: []string{"admin"}},
	})

	authService := service.NewAuthService(secret, issuer, nil, nil)
	server := gateway.MustNewServer(gwConf, gateway.WithMiddleware(
		middleware.TraceMiddleware(),
		middleware.RoutePolicyMiddleware(matcher),
		middleware.AuthMiddleware(authService),
		middleware.RouteMiddleware(),
	))
	defer server.Stop()
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
