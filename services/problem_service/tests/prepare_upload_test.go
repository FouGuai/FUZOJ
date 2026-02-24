package tests

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/problem_service/internal/handler"
	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPrepareUploadHandler(t *testing.T) {
	t.Run("success existing session", func(t *testing.T) {
		repo := &fakeProblemRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64) (bool, error) {
				return true, nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			getSessionByIdemFn: func(ctx context.Context, session sqlx.Session, problemID int64, idempotencyKey string) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:          11,
					ProblemID:   problemID,
					Version:     1,
					Bucket:      "problem-bucket",
					ObjectKey:   "problems/1/versions/1/data-pack.tar.zst",
					UploadID:    "upload-1",
					State:       repository.UploadStateUploading,
					ExpiresAt:   time.Now().Add(10 * time.Minute),
					ContentType: "application/octet-stream",
				}, nil
			},
		}
		st := &fakeStorage{}
		ctx := newTestServiceContext(repo, nil, uploadRepo, st, defaultTestConfig())
		headers := map[string]string{
			"Idempotency-Key": "k1",
		}
		body := map[string]any{
			"expected_size_bytes": 0,
			"expected_sha256":     "",
			"content_type":        "application/octet-stream",
			"created_by":          0,
			"client_type":         "",
			"upload_strategy":     "",
		}
		rr := doRequest(t, handler.PrepareUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads:prepare", body, headers, map[string]string{"id": "1"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d body: %s", rr.Code, rr.Body.String())
		}
		resp := decodeJSON[types.PrepareUploadResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.UploadId != 11 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("missing idempotency key", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.PrepareUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads:prepare", nil, nil, map[string]string{"id": "1"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.PrepareUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads:prepare", "{", nil, map[string]string{"id": "1"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("problem not found", func(t *testing.T) {
		repo := &fakeProblemRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64) (bool, error) {
				return false, nil
			},
		}
		ctx := newTestServiceContext(repo, nil, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		headers := map[string]string{"Idempotency-Key": "k2"}
		body := map[string]any{
			"expected_size_bytes": 0,
			"expected_sha256":     "",
			"content_type":        "application/octet-stream",
			"created_by":          0,
			"client_type":         "",
			"upload_strategy":     "",
		}
		rr := doRequest(t, handler.PrepareUploadHandler(ctx), http.MethodPost, "/api/v1/problems/2/data-pack/uploads:prepare", body, headers, map[string]string{"id": "2"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("storage unavailable", func(t *testing.T) {
		repo := &fakeProblemRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64) (bool, error) {
				return true, nil
			},
		}
		ctx := newTestServiceContext(repo, nil, &fakeUploadRepo{}, nil, defaultTestConfig())
		headers := map[string]string{"Idempotency-Key": "k1"}
		body := map[string]any{
			"expected_size_bytes": 0,
			"expected_sha256":     "",
			"content_type":        "application/octet-stream",
			"created_by":          0,
			"client_type":         "",
			"upload_strategy":     "",
		}
		rr := doRequest(t, handler.PrepareUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads:prepare", body, headers, map[string]string{"id": "1"})
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ServiceUnavailable) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("bucket empty", func(t *testing.T) {
		repo := &fakeProblemRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64) (bool, error) {
				return true, nil
			},
		}
		cfg := defaultTestConfig()
		cfg.MinIO.Bucket = ""
		ctx := newTestServiceContext(repo, nil, &fakeUploadRepo{}, &fakeStorage{}, cfg)
		headers := map[string]string{"Idempotency-Key": "k1"}
		body := map[string]any{
			"expected_size_bytes": 0,
			"expected_sha256":     "",
			"content_type":        "application/octet-stream",
			"created_by":          0,
			"client_type":         "",
			"upload_strategy":     "",
		}
		rr := doRequest(t, handler.PrepareUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads:prepare", body, headers, map[string]string{"id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InternalServerError) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &fakeProblemRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64) (bool, error) {
				return true, nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			getSessionByIdemFn: func(ctx context.Context, session sqlx.Session, problemID int64, idempotencyKey string) (repository.UploadSession, error) {
				return repository.UploadSession{}, errors.New("db error")
			},
		}
		ctx := newTestServiceContext(repo, nil, uploadRepo, &fakeStorage{}, defaultTestConfig())
		headers := map[string]string{"Idempotency-Key": "k1"}
		body := map[string]any{
			"expected_size_bytes": 0,
			"expected_sha256":     "",
			"content_type":        "application/octet-stream",
			"created_by":          0,
			"client_type":         "",
			"upload_strategy":     "",
		}
		rr := doRequest(t, handler.PrepareUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads:prepare", body, headers, map[string]string{"id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.DatabaseError) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
