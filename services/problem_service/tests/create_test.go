package tests

import (
	"context"
	"errors"
	"net/http"
	"testing"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/problem_service/internal/handler"
	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestCreateHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeProblemRepo{
			createFn: func(ctx context.Context, session sqlx.Session, problem *repository.Problem) (int64, error) {
				return 123, nil
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, defaultTestConfig())
		req := types.CreateProblemRequest{
			Title:   "Hello",
			OwnerId: 7,
		}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/problems", req, nil, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.CreateProblemResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.Id != 123 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/problems", "{", nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("empty title", func(t *testing.T) {
		repo := &fakeProblemRepo{
			createFn: func(ctx context.Context, session sqlx.Session, problem *repository.Problem) (int64, error) {
				return 0, nil
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, defaultTestConfig())
		req := types.CreateProblemRequest{
			Title:   "",
			OwnerId: 7,
		}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/problems", req, nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &fakeProblemRepo{
			createFn: func(ctx context.Context, session sqlx.Session, problem *repository.Problem) (int64, error) {
				return 0, errors.New("create failed")
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, defaultTestConfig())
		req := types.CreateProblemRequest{
			Title:   "Hello",
			OwnerId: 7,
		}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/problems", req, nil, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemCreateFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
