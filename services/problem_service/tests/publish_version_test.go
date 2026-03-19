package tests

import (
	"context"
	"errors"
	"net/http"
	"testing"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/problem_service/internal/handler"
	"fuzoj/services/problem_service/internal/repository"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPublishVersionHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var publishedProblemID int64
		var publishedVersion int32
		statementRepo := &fakeStatementRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error) {
				return true, nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			publishVersionFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) error {
				return nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		ctx.MetaPublisher = &fakeMetaPublisher{
			publishFn: func(ctx context.Context, problemID int64, version int32) error {
				publishedProblemID = problemID
				publishedVersion = version
				return nil
			},
		}
		rr := doRequest(t, handler.PublishVersionHandler(ctx), http.MethodPost, "/api/v1/problems/1/versions/2/publish", nil, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if publishedProblemID != 1 || publishedVersion != 2 {
			t.Fatalf("unexpected invalidation publish: problem_id=%d version=%d", publishedProblemID, publishedVersion)
		}
	})

	t.Run("publish invalidation failure does not fail request", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error) {
				return true, nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			publishVersionFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) error {
				return nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		ctx.MetaPublisher = &fakeMetaPublisher{
			publishFn: func(ctx context.Context, problemID int64, version int32) error {
				return errors.New("pubsub unavailable")
			},
		}
		rr := doRequest(t, handler.PublishVersionHandler(ctx), http.MethodPost, "/api/v1/problems/1/versions/2/publish", nil, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.PublishVersionHandler(ctx), http.MethodPost, "/api/v1/problems/0/versions/0/publish", nil, nil, map[string]string{"id": "0", "version": "0"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error) {
				return true, nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			publishVersionFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) error {
				return repository.ErrProblemVersionNotFound
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.PublishVersionHandler(ctx), http.MethodPost, "/api/v1/problems/1/versions/2/publish", nil, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.NotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not ready", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error) {
				return true, nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			publishVersionFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) error {
				return repository.ErrProblemVersionNotReady
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.PublishVersionHandler(ctx), http.MethodPost, "/api/v1/problems/1/versions/2/publish", nil, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemVersionNotReadyToPublish) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error) {
				return true, nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			publishVersionFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) error {
				return errors.New("publish failed")
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.PublishVersionHandler(ctx), http.MethodPost, "/api/v1/problems/1/versions/2/publish", nil, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.DatabaseError) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("statement missing", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			existsFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error) {
				return false, nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			publishVersionFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) error {
				return nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.PublishVersionHandler(ctx), http.MethodPost, "/api/v1/problems/1/versions/2/publish", nil, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemStatementNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
