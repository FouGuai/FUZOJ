package gateway_test

import (
	"net/http"
	"testing"

	"fuzoj/services/gateway_service/internal/middleware"
)

func TestTraceMiddleware(t *testing.T) {
	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, middleware.TraceMiddleware())

	rec, _, err := performRequest(http.HandlerFunc(handler), http.MethodGet, "/trace", nil)
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	traceID := rec.Header().Get("X-Trace-Id")
	if traceID == "" {
		t.Fatalf("expected trace id header")
	}
}

func TestTraceMiddlewarePreservesRequestID(t *testing.T) {
	handler := applyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, middleware.TraceMiddleware())

	rec, _, err := performRequest(http.HandlerFunc(handler), http.MethodGet, "/trace", map[string]string{"X-Request-Id": "req-123"})
	if err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if rec.Header().Get("X-Request-Id") != "req-123" {
		t.Fatalf("expected request id header")
	}
}
