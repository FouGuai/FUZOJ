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

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestAbortUploadHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
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
			markAbortedFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
				return nil
			},
		}
		st := &fakeStorage{
			abortMultipartFn: func(ctx context.Context, bucket, objectKey, uploadID string) error {
				return nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, st, defaultTestConfig())
		rr := doRequest(t, handler.AbortUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/abort", nil, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.AbortUploadHandler(ctx), http.MethodPost, "/api/v1/problems/0/data-pack/uploads/0/abort", nil, nil, map[string]string{"id": "0", "upload_id": "0"})
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
		rr := doRequest(t, handler.AbortUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/9/abort", nil, nil, map[string]string{"id": "1", "upload_id": "9"})
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
		rr := doRequest(t, handler.AbortUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/abort", nil, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemUploadStateInvalid) {
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
			abortMultipartFn: func(ctx context.Context, bucket, objectKey, uploadID string) error {
				return errors.New("abort failed")
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, uploadRepo, st, defaultTestConfig())
		rr := doRequest(t, handler.AbortUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/abort", nil, nil, map[string]string{"id": "1", "upload_id": "1"})
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
		rr := doRequest(t, handler.AbortUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/abort", nil, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ServiceUnavailable) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
