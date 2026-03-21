package tests

import (
	"context"
	"net/http"
	"testing"
	"time"

	"fuzoj/services/problem_service/internal/handler"
	"fuzoj/services/problem_service/internal/logic"
	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/types"
)

func TestListLogicListDefaultsAndHasMore(t *testing.T) {
	ctx := newTestServiceContext(&fakeProblemRepo{
		listPublishedFn: func(ctx context.Context, cursorID int64, limit int) ([]repository.ProblemListItem, error) {
			if cursorID != 0 {
				t.Fatalf("unexpected cursorID: %d", cursorID)
			}
			if limit != 9 {
				t.Fatalf("unexpected limit: %d", limit)
			}
			return []repository.ProblemListItem{
				{ProblemID: 9, Title: "A", Version: 3, UpdatedAt: time.Unix(1700000000, 0)},
				{ProblemID: 8, Title: "B", Version: 2, UpdatedAt: time.Unix(1700000100, 0)},
				{ProblemID: 7, Title: "C", Version: 1, UpdatedAt: time.Unix(1700000200, 0)},
				{ProblemID: 6, Title: "D", Version: 1, UpdatedAt: time.Unix(1700000300, 0)},
				{ProblemID: 5, Title: "E", Version: 1, UpdatedAt: time.Unix(1700000400, 0)},
				{ProblemID: 4, Title: "F", Version: 1, UpdatedAt: time.Unix(1700000500, 0)},
				{ProblemID: 3, Title: "G", Version: 1, UpdatedAt: time.Unix(1700000600, 0)},
				{ProblemID: 2, Title: "H", Version: 1, UpdatedAt: time.Unix(1700000700, 0)},
				{ProblemID: 1, Title: "I", Version: 1, UpdatedAt: time.Unix(1700000800, 0)},
			}, nil
		},
	}, nil, nil, nil, defaultTestConfig())

	resp, err := logic.NewListLogic(context.Background(), ctx).List(&types.ListProblemsRequest{Limit: 8})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !resp.Data.HasMore {
		t.Fatalf("expected has_more=true")
	}
	if resp.Data.NextCursor != "2" {
		t.Fatalf("unexpected next_cursor: %s", resp.Data.NextCursor)
	}
	if len(resp.Data.Items) != 8 {
		t.Fatalf("unexpected item count: %d", len(resp.Data.Items))
	}
}

func TestListLogicListInvalidCursor(t *testing.T) {
	ctx := newTestServiceContext(&fakeProblemRepo{}, nil, nil, nil, defaultTestConfig())
	_, err := logic.NewListLogic(context.Background(), ctx).List(&types.ListProblemsRequest{Cursor: "abc"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListHandler(t *testing.T) {
	ctx := newTestServiceContext(&fakeProblemRepo{
		listPublishedFn: func(ctx context.Context, cursorID int64, limit int) ([]repository.ProblemListItem, error) {
			if cursorID != 10 {
				t.Fatalf("unexpected cursorID: %d", cursorID)
			}
			if limit != 3 {
				t.Fatalf("unexpected limit: %d", limit)
			}
			return []repository.ProblemListItem{
				{ProblemID: 9, Title: "Two Sum", Version: 5, UpdatedAt: time.Unix(1700000000, 0)},
				{ProblemID: 8, Title: "Add Two Numbers", Version: 2, UpdatedAt: time.Unix(1700000100, 0)},
			}, nil
		},
	}, nil, nil, nil, defaultTestConfig())
	rr := doRequest(t, handler.ListHandler(ctx), http.MethodGet, "/api/v1/problems?cursor=10&limit=2", nil, nil, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
	resp := decodeJSON[types.ListProblemsResponse](t, rr.Body)
	if resp.Data.HasMore {
		t.Fatalf("expected has_more=false")
	}
	if resp.Data.NextCursor != "" {
		t.Fatalf("unexpected next_cursor: %s", resp.Data.NextCursor)
	}
	if len(resp.Data.Items) != 2 || resp.Data.Items[0].ProblemId != 9 {
		t.Fatalf("unexpected items: %+v", resp.Data.Items)
	}
}
