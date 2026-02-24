package tests

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"fuzoj/internal/common/storage"
	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/problem_service/internal/handler"
	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestCompleteUploadHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:                uploadSessionID,
					ProblemID:         1,
					Version:           2,
					Bucket:            "problem-bucket",
					ObjectKey:         "problems/1/versions/2/data-pack.tar.zst",
					UploadID:          "upload-1",
					State:             repository.UploadStateUploading,
					ExpectedSizeBytes: 10,
					ExpiresAt:         time.Now().Add(10 * time.Minute),
				}, nil
			},
			getProblemVersionIDFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (int64, error) {
				return 99, nil
			},
			updateProblemDraftMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) error {
				return nil
			},
			upsertManifestFn: func(ctx context.Context, session sqlx.Session, problemVersionID int64, manifestJSON []byte) error {
				return nil
			},
			upsertDataPackFn: func(ctx context.Context, session sqlx.Session, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error {
				return nil
			},
			markCompletedFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
				return nil
			},
		}
		st := &fakeStorage{
			completeMultipartFn: func(ctx context.Context, bucket, objectKey, uploadID string, parts []storage.CompletedPart) (string, error) {
				return "etag", nil
			},
			statObjectFn: func(ctx context.Context, bucket, objectKey string) (storage.ObjectStat, error) {
				return storage.ObjectStat{SizeBytes: 10}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, uploadRepo, st, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{{PartNumber: 1, ETag: "etag"}},
			"manifest_json":  `{"name":"x"}`,
			"config_json":    `{"version":1}`,
			"manifest_hash":  "mh",
			"data_pack_hash": "dh",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/complete", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.CompleteUploadResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.Version != 2 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{},
			"manifest_json":  "",
			"config_json":    "",
			"manifest_hash":  "",
			"data_pack_hash": "",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/complete", body, nil, map[string]string{"id": "1", "upload_id": "1"})
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
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{{PartNumber: 1, ETag: "etag"}},
			"manifest_json":  "{}",
			"config_json":    "{}",
			"manifest_hash":  "mh",
			"data_pack_hash": "dh",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/9/complete", body, nil, map[string]string{"id": "1", "upload_id": "9"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemUploadNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("completed idempotent", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:        uploadSessionID,
					ProblemID: 1,
					Version:   3,
					State:     repository.UploadStateCompleted,
				}, nil
			},
			getProblemVersionMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemVersionMeta, error) {
				return repository.ProblemVersionMeta{
					ProblemID:    1,
					Version:      3,
					ManifestHash: "mh",
					DataPackKey:  "key",
					DataPackHash: "dh",
				}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{{PartNumber: 1, ETag: "etag"}},
			"manifest_json":  "{}",
			"config_json":    "{}",
			"manifest_hash":  "mh",
			"data_pack_hash": "dh",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/complete", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.CompleteUploadResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.Version != 3 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("state invalid", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:        uploadSessionID,
					ProblemID: 1,
					State:     repository.UploadStateAborted,
				}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{{PartNumber: 1, ETag: "etag"}},
			"manifest_json":  "{}",
			"config_json":    "{}",
			"manifest_hash":  "mh",
			"data_pack_hash": "dh",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/complete", body, nil, map[string]string{"id": "1", "upload_id": "1"})
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
					State:     repository.UploadStateUploading,
					UploadID:  "upload-1",
					ExpiresAt: time.Now().Add(-time.Minute),
				}, nil
			},
			markExpiredFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
				return nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{{PartNumber: 1, ETag: "etag"}},
			"manifest_json":  "{}",
			"config_json":    "{}",
			"manifest_hash":  "mh",
			"data_pack_hash": "dh",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/complete", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemUploadExpired) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("hash conflict", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:             uploadSessionID,
					ProblemID:      1,
					State:          repository.UploadStateUploading,
					UploadID:       "upload-1",
					ExpectedSHA256: "hash-a",
					ExpiresAt:      time.Now().Add(10 * time.Minute),
				}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{{PartNumber: 1, ETag: "etag"}},
			"manifest_json":  "{}",
			"config_json":    "{}",
			"manifest_hash":  "mh",
			"data_pack_hash": "hash-b",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/complete", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemUploadConflict) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("storage error", func(t *testing.T) {
		uploadRepo := &fakeUploadRepo{
			getSessionByIDFn: func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
				return repository.UploadSession{
					ID:        uploadSessionID,
					ProblemID: 1,
					State:     repository.UploadStateUploading,
					UploadID:  "upload-1",
					Bucket:    "problem-bucket",
					ObjectKey: "key",
					ExpiresAt: time.Now().Add(10 * time.Minute),
				}, nil
			},
		}
		st := &fakeStorage{
			completeMultipartFn: func(ctx context.Context, bucket, objectKey, uploadID string, parts []storage.CompletedPart) (string, error) {
				return "", errors.New("upload failed")
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, uploadRepo, st, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{{PartNumber: 1, ETag: "etag"}},
			"manifest_json":  "{}",
			"config_json":    "{}",
			"manifest_hash":  "mh",
			"data_pack_hash": "dh",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/complete", body, nil, map[string]string{"id": "1", "upload_id": "1"})
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
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, uploadRepo, nil, defaultTestConfig())
		body := map[string]any{
			"parts":          []types.CompletedPartInput{{PartNumber: 1, ETag: "etag"}},
			"manifest_json":  "{}",
			"config_json":    "{}",
			"manifest_hash":  "mh",
			"data_pack_hash": "dh",
		}
		rr := doRequest(t, handler.CompleteUploadHandler(ctx), http.MethodPost, "/api/v1/problems/1/data-pack/uploads/1/complete", body, nil, map[string]string{"id": "1", "upload_id": "1"})
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ServiceUnavailable) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
