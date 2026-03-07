package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fuzoj/services/contest_service/internal/config"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/svc"

	"github.com/zeromicro/go-zero/rest/pathvar"
)

type errorResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	TraceId string            `json:"trace_id,omitempty"`
}

func newTestServiceContext(contestRepo repository.ContestRepository, cfg config.Config) *svc.ServiceContext {
	ctx := &svc.ServiceContext{
		Config:       cfg,
		ContestStore: contestRepo,
	}
	return ctx
}

func doRequest(t *testing.T, handler http.HandlerFunc, method, path string, body any, headers map[string]string, pathVars map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if method == http.MethodGet {
		reader = nil
	} else {
		switch val := body.(type) {
		case nil:
			reader = nil
		case string:
			reader = bytes.NewBufferString(val)
		default:
			b, err := json.Marshal(val)
			if err != nil {
				t.Fatalf("marshal body failed: %v", err)
			}
			reader = bytes.NewBuffer(b)
		}
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if len(pathVars) > 0 {
		req = pathvar.WithVars(req, pathVars)
	}
	if method == http.MethodGet && body != nil {
		if payload, ok := body.(map[string]string); ok {
			q := req.URL.Query()
			for k, v := range payload {
				q.Set(k, v)
			}
			req.URL.RawQuery = q.Encode()
		}
	}

	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func decodeJSON[T any](t *testing.T, body io.Reader) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		t.Fatalf("decode json failed: %v", err)
	}
	return out
}

func defaultTestConfig() config.Config {
	return config.Config{
		Contest: config.ContestConfig{
			DefaultPageSize: 50,
			MaxPageSize:     200,
		},
		Timeouts: config.TimeoutConfig{
			DB: time.Second,
		},
	}
}
