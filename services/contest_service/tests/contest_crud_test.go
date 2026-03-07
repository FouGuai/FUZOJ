package tests

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/handler"
	"fuzoj/services/contest_service/internal/logic"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/types"
)

type fakeContestStoreRepo struct {
	createFn     func(ctx context.Context, input repository.ContestCreateInput) error
	getFn        func(ctx context.Context, contestID string) (repository.ContestDetail, error)
	listFn       func(ctx context.Context, filter repository.ContestListFilter) ([]repository.ContestListItem, int, error)
	updateFn     func(ctx context.Context, contestID string, update repository.ContestUpdate) error
	invalidateFn func(ctx context.Context, contestID string) error
}

func (f *fakeContestStoreRepo) Create(ctx context.Context, input repository.ContestCreateInput) error {
	if f.createFn != nil {
		return f.createFn(ctx, input)
	}
	return nil
}

func (f *fakeContestStoreRepo) Get(ctx context.Context, contestID string) (repository.ContestDetail, error) {
	if f.getFn != nil {
		return f.getFn(ctx, contestID)
	}
	return repository.ContestDetail{}, repository.ErrContestNotFound
}

func (f *fakeContestStoreRepo) List(ctx context.Context, filter repository.ContestListFilter) ([]repository.ContestListItem, int, error) {
	if f.listFn != nil {
		return f.listFn(ctx, filter)
	}
	return nil, 0, nil
}

func (f *fakeContestStoreRepo) Update(ctx context.Context, contestID string, update repository.ContestUpdate) error {
	if f.updateFn != nil {
		return f.updateFn(ctx, contestID, update)
	}
	return nil
}

func (f *fakeContestStoreRepo) InvalidateDetailCache(ctx context.Context, contestID string) error {
	if f.invalidateFn != nil {
		return f.invalidateFn(ctx, contestID)
	}
	return nil
}

func TestContestCreateHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeContestStoreRepo{}
		ctx := newTestServiceContext(repo, defaultTestConfig())
		req := types.CreateContestRequest{
			Title:       "Weekly",
			Description: "desc",
			Visibility:  "public",
			OwnerId:     1,
			OrgId:       2,
			StartAt:     time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			EndAt:       time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
		}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/contests", req, nil, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.CreateContestResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.ContestId == "" {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeContestStoreRepo{}, defaultTestConfig())
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/contests", "{", nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("empty title", func(t *testing.T) {
		ctx := newTestServiceContext(&fakeContestStoreRepo{}, defaultTestConfig())
		req := types.CreateContestRequest{
			Title:   "",
			StartAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			EndAt:   time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
		}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/contests", req, nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ValidationFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &fakeContestStoreRepo{
			createFn: func(ctx context.Context, input repository.ContestCreateInput) error {
				return errors.New("create failed")
			},
		}
		ctx := newTestServiceContext(repo, defaultTestConfig())
		req := types.CreateContestRequest{
			Title:   "Hello",
			StartAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			EndAt:   time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
		}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/contests", req, nil, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ContestCreateFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}

func TestContestGetHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeContestStoreRepo{
			getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
				return repository.ContestDetail{
					ContestID:   contestID,
					Title:       "Weekly",
					Description: "desc",
					Status:      "draft",
					Visibility:  "public",
					OwnerID:     1,
					OrgID:       2,
					StartAt:     time.Now().Add(time.Hour),
					EndAt:       time.Now().Add(2 * time.Hour),
					RuleJSON:    `{"rule_type":"icpc"}`,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}, nil
			},
		}
		ctx := newTestServiceContext(repo, defaultTestConfig())
		rr := doRequest(t, handler.GetHandler(ctx), http.MethodGet, "/api/v1/contests/abc", nil, nil, map[string]string{"id": "abc"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.GetContestResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.ContestId != "abc" {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &fakeContestStoreRepo{
			getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
				return repository.ContestDetail{}, repository.ErrContestNotFound
			},
		}
		ctx := newTestServiceContext(repo, defaultTestConfig())
		rr := doRequest(t, handler.GetHandler(ctx), http.MethodGet, "/api/v1/contests/missing", nil, nil, map[string]string{"id": "missing"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ContestNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}

func TestContestListHandler(t *testing.T) {
	repo := &fakeContestStoreRepo{
		listFn: func(ctx context.Context, filter repository.ContestListFilter) ([]repository.ContestListItem, int, error) {
			items := []repository.ContestListItem{
				{ContestID: "c1", Title: "A", Status: "draft", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), RuleJSON: `{"rule_type":"icpc"}`},
			}
			return items, 1, nil
		},
	}
	ctx := newTestServiceContext(repo, defaultTestConfig())
	resp, err := logic.NewListLogic(context.Background(), ctx).List(&types.ListContestsRequest{Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != int(pkgerrors.Success) || len(resp.Data.Items) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestContestUpdateHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeContestStoreRepo{
			getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
				return repository.ContestDetail{
					ContestID: contestID,
					Title:     "Old",
					Status:    "draft",
					StartAt:   time.Now().Add(time.Hour),
					EndAt:     time.Now().Add(2 * time.Hour),
				}, nil
			},
			updateFn: func(ctx context.Context, contestID string, update repository.ContestUpdate) error {
				return nil
			},
		}
		ctx := newTestServiceContext(repo, defaultTestConfig())
		req := types.UpdateContestRequest{
			Id:    "abc",
			Title: "New",
		}
		rr := doRequest(t, handler.UpdateHandler(ctx), http.MethodPut, "/api/v1/contests/abc", req, nil, map[string]string{"id": "abc"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.SuccessResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid time", func(t *testing.T) {
		repo := &fakeContestStoreRepo{
			getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
				return repository.ContestDetail{
					ContestID: contestID,
					Title:     "Old",
					Status:    "draft",
					StartAt:   time.Now().Add(time.Hour),
					EndAt:     time.Now().Add(2 * time.Hour),
				}, nil
			},
		}
		ctx := newTestServiceContext(repo, defaultTestConfig())
		req := types.UpdateContestRequest{
			Id:      "abc",
			StartAt: "bad-time",
		}
		rr := doRequest(t, handler.UpdateHandler(ctx), http.MethodPut, "/api/v1/contests/abc", req, nil, map[string]string{"id": "abc"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ValidationFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &fakeContestStoreRepo{
			getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
				return repository.ContestDetail{}, repository.ErrContestNotFound
			},
		}
		ctx := newTestServiceContext(repo, defaultTestConfig())
		req := types.UpdateContestRequest{Id: "missing", Title: "New"}
		rr := doRequest(t, handler.UpdateHandler(ctx), http.MethodPut, "/api/v1/contests/missing", req, nil, map[string]string{"id": "missing"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ContestNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &fakeContestStoreRepo{
			getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
				return repository.ContestDetail{
					ContestID: contestID,
					Title:     "Old",
					Status:    "draft",
					StartAt:   time.Now().Add(time.Hour),
					EndAt:     time.Now().Add(2 * time.Hour),
				}, nil
			},
			updateFn: func(ctx context.Context, contestID string, update repository.ContestUpdate) error {
				return errors.New("update failed")
			},
		}
		ctx := newTestServiceContext(repo, defaultTestConfig())
		req := types.UpdateContestRequest{Id: "abc", Title: "New"}
		rr := doRequest(t, handler.UpdateHandler(ctx), http.MethodPut, "/api/v1/contests/abc", req, nil, map[string]string{"id": "abc"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ContestUpdateFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
