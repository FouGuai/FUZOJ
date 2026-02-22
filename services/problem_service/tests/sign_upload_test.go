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

func TestSignUploadHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:          uploadSessionID,
					ProblemID:   1,
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
		st := &fakeStorage{
			presignUploadPartFn: func(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
				return "https://example.com/part", nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, st, defaultTestConfig())
		body := map[string]any{
			"part_numbers": []int{1, 2},
		}
		rr := doRequest(t, handler.SignUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/2/sign", body, nil, map[string]string{"id": "1", "upload_id": "2"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.SignPartsResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || len(resp.Data.Urls) != 2 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"part_numbers": []int{},
		}
		rr := doRequest(t, handler.SignUploadHandler(ctx), http.MethodPost, "/api/v1/problems/0/data-pack/uploads/0/sign", body, nil, map[string]string{"id": "0", "upload_id": "0"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("part number invalid", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:        uploadSessionID,
					ProblemID: 1,
					Bucket:    "problem-bucket",
					ObjectKey: "problems/1/versions/1/data-pack.tar.zst",
					UploadID:  "upload-1",
					State:     repository.UploadStateUploading,
					ExpiresAt: time.Now().Add(10 * time.Minute),
				}, nil
			},
		}
		st := &fakeStorage{
			presignUploadPartFn: func(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
				return "ok", nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, st, defaultTestConfig())
		body := map[string]any{
			"part_numbers": []int{0},
		}
		rr := doRequest(t, handler.SignUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/sign", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{}, repository.ErrUploadNotFound
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"part_numbers": []int{1},
		}
		rr := doRequest(t, handler.SignUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/99/sign", body, nil, map[string]string{"id": "1", "upload_id": "99"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemUploadNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("state invalid", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:        uploadSessionID,
					ProblemID: 1,
					State:     repository.UploadStateCompleted,
				}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"part_numbers": []int{1},
		}
		rr := doRequest(t, handler.SignUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/sign", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemUploadStateInvalid) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("expired", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:        uploadSessionID,
					ProblemID: 1,
					UploadID:  "upload-1",
					State:     repository.UploadStateUploading,
					ExpiresAt: time.Now().Add(-time.Minute),
				}, nil
			},
			markExpiredFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
				return nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"part_numbers": []int{1},
		}
		rr := doRequest(t, handler.SignUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/sign", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemUploadExpired) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("storage error", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:        uploadSessionID,
					ProblemID: 1,
					UploadID:  "upload-1",
					Bucket:    "problem-bucket",
					ObjectKey: "key",
					State:     repository.UploadStateUploading,
					ExpiresAt: time.Now().Add(10 * time.Minute),
				}, nil
			},
		}
		st := &fakeStorage{
			presignUploadPartFn: func(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
				return "", errors.New("presign failed")
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, st, defaultTestConfig())
		body := map[string]any{
			"part_numbers": []int{1},
		}
		rr := doRequest(t, handler.SignUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/sign", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemUploadObjectStorageFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("storage unavailable", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:        uploadSessionID,
					ProblemID: 1,
					State:     repository.UploadStateUploading,
				}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, nil, defaultTestConfig())
		body := map[string]any{
			"part_numbers": []int{1},
		}
		rr := doRequest(t, handler.SignUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/sign", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ServiceUnavailable) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
