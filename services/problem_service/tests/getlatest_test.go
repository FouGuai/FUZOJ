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

func TestGetLatestHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeProblemRepo{
			getLatestMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemLatestMeta, error) {
				return repository.ProblemLatestMeta{
					ProblemID:    10,
					Version:      2,
					ManifestHash: "m1",
					DataPackKey:  "k1",
					DataPackHash: "h1",
					UpdatedAt:    time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC),
				}, nil
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.GetLatestHandler(ctx), http.MethodGet, "/api/v1/problems/10/latest", nil, nil, map[string]string{"id": "10"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.LatestMetaResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.ProblemId != 10 {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if resp.Data.UpdatedAt == "" {
			t.Fatalf("missing updated_at")
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.GetLatestHandler(ctx), http.MethodGet, "/api/v1/problems/0/latest", nil, nil, map[string]string{"id": "0"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &fakeProblemRepo{
			getLatestMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemLatestMeta, error) {
				return repository.ProblemLatestMeta{}, repository.ErrProblemNotFound
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.GetLatestHandler(ctx), http.MethodGet, "/api/v1/problems/99/latest", nil, nil, map[string]string{"id": "99"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &fakeProblemRepo{
			getLatestMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemLatestMeta, error) {
				return repository.ProblemLatestMeta{}, errors.New("db error")
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.GetLatestHandler(ctx), http.MethodGet, "/api/v1/problems/1/latest", nil, nil, map[string]string{"id": "1"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.DatabaseError) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
