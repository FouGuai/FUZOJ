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

func TestDeleteHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeProblemRepo{
			deleteFn: func(ctx context.Context, session sqlx.Session, problemID int64) error {
				return nil
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.DeleteHandler(ctx), http.MethodDelete, "/api/v1/problems/1", nil, nil, map[string]string{"id": "1"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeProblemRepo{}, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.DeleteHandler(ctx), http.MethodDelete, "/api/v1/problems/0", nil, nil, map[string]string{"id": "0"})
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
			deleteFn: func(ctx context.Context, session sqlx.Session, problemID int64) error {
				return repository.ErrProblemNotFound
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.DeleteHandler(ctx), http.MethodDelete, "/api/v1/problems/10", nil, nil, map[string]string{"id": "10"})
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
			deleteFn: func(ctx context.Context, session sqlx.Session, problemID int64) error {
				return errors.New("delete failed")
			},
		}
		ctx := newTestServiceContext(repo, nil, nil, defaultTestConfig())
		rr := doRequest(t, handler.DeleteHandler(ctx), http.MethodDelete, "/api/v1/problems/10", nil, nil, map[string]string{"id": "10"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ProblemDeleteFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
