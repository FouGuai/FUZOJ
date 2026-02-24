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

func TestGetStatementHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			getLatestFn: func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemStatement, error) {
				return repository.ProblemStatement{
					ProblemID:   1,
					Version:     2,
					StatementMd: "## Title",
					UpdatedAt:   time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC),
				}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.GetStatementHandler(ctx), http.MethodGet, "/api/v1/problems/1/statement", nil, nil, map[string]string{"id": "1"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.StatementResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.ProblemId != 1 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, &fakeStatementRepo{}, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.GetStatementHandler(ctx), http.MethodGet, "/api/v1/problems/0/statement", nil, nil, map[string]string{"id": "0"})
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
			getLatestFn: func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemStatement, error) {
				return repository.ProblemStatement{}, repository.ErrProblemStatementNotFound
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.GetStatementHandler(ctx), http.MethodGet, "/api/v1/problems/9/statement", nil, nil, map[string]string{"id": "9"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemStatementNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}

func TestGetStatementVersionHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			getByVersionFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemStatement, error) {
				return repository.ProblemStatement{
					ProblemID:   1,
					Version:     version,
					StatementMd: "## Title",
					UpdatedAt:   time.Now(),
				}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.GetStatementVersionHandler(ctx), http.MethodGet, "/api/v1/problems/1/versions/2/statement", nil, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.StatementResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.Version != 2 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			getByVersionFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemStatement, error) {
				return repository.ProblemStatement{}, repository.ErrProblemStatementNotFound
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		rr := doRequest(t, handler.GetStatementVersionHandler(ctx), http.MethodGet, "/api/v1/problems/1/versions/2/statement", nil, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemStatementNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}

func TestUpdateStatementHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			upsertFn: func(ctx context.Context, session sqlx.Session, statement repository.ProblemStatement, problemVersionID int64) error {
				return nil
			},
		}
		uploadRepo := &fakeUploadRepo{
			getProblemVersionMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemVersionMeta, error) {
				return repository.ProblemVersionMeta{
					ProblemID: problemID,
					Version:   version,
					State:     repository.ProblemVersionStateDraft,
				}, nil
			},
			getProblemVersionIDFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (int64, error) {
				return 100, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{"statement_md": "## Hello"}
		rr := doRequest(t, handler.UpdateStatementHandler(ctx), http.MethodPut, "/api/v1/problems/1/versions/2/statement", body, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, &fakeStatementRepo{}, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{"statement_md": ""}
		rr := doRequest(t, handler.UpdateStatementHandler(ctx), http.MethodPut, "/api/v1/problems/1/versions/2/statement", body, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not editable", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{}
		uploadRepo := &fakeUploadRepo{
			getProblemVersionMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemVersionMeta, error) {
				return repository.ProblemVersionMeta{
					ProblemID: problemID,
					Version:   version,
					State:     repository.ProblemVersionStatePublished,
				}, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{"statement_md": "## Hello"}
		rr := doRequest(t, handler.UpdateStatementHandler(ctx), http.MethodPut, "/api/v1/problems/1/versions/2/statement", body, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusConflict {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemStatementNotEditable) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		statementRepo := &fakeStatementRepo{
			upsertFn: func(ctx context.Context, session sqlx.Session, statement repository.ProblemStatement, problemVersionID int64) error {
				return errors.New("db error")
			},
		}
		uploadRepo := &fakeUploadRepo{
			getProblemVersionMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemVersionMeta, error) {
				return repository.ProblemVersionMeta{
					ProblemID: problemID,
					Version:   version,
					State:     repository.ProblemVersionStateDraft,
				}, nil
			},
			getProblemVersionIDFn: func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (int64, error) {
				return 101, nil
			},
		}
		ctx := newTestServiceContext(&fakeProblemRepo{}, statementRepo, uploadRepo, &fakeStorage{}, defaultTestConfig())
		body := map[string]any{"statement_md": "## Hello"}
		rr := doRequest(t, handler.UpdateStatementHandler(ctx), http.MethodPut, "/api/v1/problems/1/versions/2/statement", body, nil, map[string]string{"id": "1", "version": "2"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemStatementUpdateFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
