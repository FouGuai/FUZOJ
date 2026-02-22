package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fuzoj/services/problem_service/internal/config"
	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/svc"

	"fuzoj/internal/common/storage"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

type errorResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	TraceId string            `json:"trace_id,omitempty"`
}

func newTestServiceContext(problemRepo repository.ProblemRepository, uploadRepo repository.ProblemUploadRepository, storage storage.ObjectStorage, cfg config.Config) *svc.ServiceContext {
	ctx := &svc.ServiceContext{
		Config:      cfg,
		ProblemRepo: problemRepo,
		UploadRepo:  uploadRepo,
		Conn:        nil,
	}
	ctx.Storage = storage
	return ctx
}

func doRequest(t *testing.T, handler http.HandlerFunc, method, path string, body any, headers map[string]string, pathVars map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
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

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if len(pathVars) > 0 {
		req = pathvar.WithVars(req, pathVars)
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
		Upload: config.UploadConfig{
			KeyPrefix:     "problems",
			PartSizeBytes: 16 * 1024 * 1024,
			SessionTTL:    2 * time.Hour,
			PresignTTL:    15 * time.Minute,
		},
		MinIO: config.MinIOConfig{
			Bucket: "problem-bucket",
		},
	}
}
