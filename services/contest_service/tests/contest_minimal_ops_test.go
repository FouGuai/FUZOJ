package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/config"
	"fuzoj/services/contest_service/internal/logic"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"
)

type fakeContestProblemStore struct {
	items map[int64]repository.ContestProblemItem
}

func (f *fakeContestProblemStore) Upsert(ctx context.Context, contestID string, item repository.ContestProblemItem) error {
	if f.items == nil {
		f.items = make(map[int64]repository.ContestProblemItem)
	}
	f.items[item.ProblemID] = item
	return nil
}

func (f *fakeContestProblemStore) Update(ctx context.Context, contestID string, item repository.ContestProblemItem) error {
	if f.items == nil {
		return repository.ErrContestProblemNotFound
	}
	if _, ok := f.items[item.ProblemID]; !ok {
		return repository.ErrContestProblemNotFound
	}
	f.items[item.ProblemID] = item
	return nil
}

func (f *fakeContestProblemStore) Remove(ctx context.Context, contestID string, problemID int64) error {
	if f.items != nil {
		delete(f.items, problemID)
	}
	return nil
}

func (f *fakeContestProblemStore) List(ctx context.Context, contestID string) ([]repository.ContestProblemItem, error) {
	resp := make([]repository.ContestProblemItem, 0, len(f.items))
	for _, item := range f.items {
		resp = append(resp, item)
	}
	return resp, nil
}

type fakeContestParticipantStore struct {
	findErr     error
	findItem    repository.ContestParticipantItem
	upsertCount int
}

func (f *fakeContestParticipantStore) Upsert(ctx context.Context, contestID string, item repository.ContestParticipantItem) error {
	f.upsertCount++
	f.findItem = item
	return nil
}

func (f *fakeContestParticipantStore) Find(ctx context.Context, contestID string, userID int64) (repository.ContestParticipantItem, error) {
	if f.findErr != nil {
		return repository.ContestParticipantItem{}, f.findErr
	}
	return f.findItem, nil
}

func (f *fakeContestParticipantStore) List(ctx context.Context, contestID string, page, pageSize int) ([]repository.ContestParticipantItem, int, error) {
	if f.findItem.UserID == 0 {
		return nil, 0, nil
	}
	return []repository.ContestParticipantItem{f.findItem}, 1, nil
}

func newOpsServiceContext(contest repository.ContestRepository, problem repository.ContestProblemStore, participant repository.ContestParticipantStore) *svc.ServiceContext {
	return &svc.ServiceContext{
		Config: config.Config{
			Contest: config.ContestConfig{
				DefaultPageSize: 20,
				MaxPageSize:     200,
			},
			Timeouts: config.TimeoutConfig{
				DB: time.Second,
			},
		},
		ContestStore:            contest,
		ContestProblemStore:     problem,
		ContestParticipantStore: participant,
	}
}

func TestPublishAndCloseLogic(t *testing.T) {
	status := "draft"
	contestStore := &fakeContestStoreRepo{
		getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
			return repository.ContestDetail{
				ContestID: contestID,
				Status:    status,
				StartAt:   time.Now().Add(-time.Hour),
				EndAt:     time.Now().Add(time.Hour),
			}, nil
		},
		updateFn: func(ctx context.Context, contestID string, update repository.ContestUpdate) error {
			if update.Status != nil {
				status = *update.Status
			}
			return nil
		},
	}
	ctx := newOpsServiceContext(contestStore, &fakeContestProblemStore{}, &fakeContestParticipantStore{})

	if _, err := logic.NewPublishLogic(context.Background(), ctx).Publish(&types.GetContestRequest{Id: "c1"}); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if status != "published" {
		t.Fatalf("expected published, got %s", status)
	}
	if _, err := logic.NewCloseLogic(context.Background(), ctx).Close(&types.GetContestRequest{Id: "c1"}); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if status != "ended" {
		t.Fatalf("expected ended, got %s", status)
	}
}

func TestRegisterLogic_Idempotent(t *testing.T) {
	contestStore := &fakeContestStoreRepo{
		getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
			return repository.ContestDetail{
				ContestID: contestID,
				Status:    "published",
				StartAt:   time.Now().Add(-time.Hour),
				EndAt:     time.Now().Add(time.Hour),
			}, nil
		},
	}
	participantStore := &fakeContestParticipantStore{
		findItem: repository.ContestParticipantItem{
			UserID:       100,
			TeamID:       "t1",
			Status:       "registered",
			RegisteredAt: time.Now(),
		},
	}
	ctx := newOpsServiceContext(contestStore, &fakeContestProblemStore{}, participantStore)

	if _, err := logic.NewRegisterLogic(context.Background(), ctx).Register(&types.RegisterContestRequest{
		Id:     "c1",
		UserId: 100,
		TeamId: "t1",
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if participantStore.upsertCount != 0 {
		t.Fatalf("expected idempotent register without upsert, got %d", participantStore.upsertCount)
	}

	participantStore.findErr = repository.ErrContestParticipantNotFound
	if _, err := logic.NewRegisterLogic(context.Background(), ctx).Register(&types.RegisterContestRequest{
		Id:     "c1",
		UserId: 101,
		TeamId: "t2",
	}); err != nil {
		t.Fatalf("register create failed: %v", err)
	}
	if participantStore.upsertCount != 1 {
		t.Fatalf("expected one upsert, got %d", participantStore.upsertCount)
	}
}

func TestProblemLogic_BasicFlow(t *testing.T) {
	contestStore := &fakeContestStoreRepo{
		getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
			return repository.ContestDetail{
				ContestID: contestID,
				Status:    "published",
				StartAt:   time.Now().Add(-time.Hour),
				EndAt:     time.Now().Add(time.Hour),
			}, nil
		},
	}
	problemStore := &fakeContestProblemStore{}
	ctx := newOpsServiceContext(contestStore, problemStore, &fakeContestParticipantStore{})

	_, err := logic.NewProblemAddLogic(context.Background(), ctx).ProblemAdd(&types.AddContestProblemRequest{
		Id:        "c1",
		ProblemId: 1001,
		Order:     1,
		Score:     100,
		Visible:   true,
		Version:   3,
	})
	if err != nil {
		t.Fatalf("problem add failed: %v", err)
	}

	listResp, err := logic.NewProblemListLogic(context.Background(), ctx).ProblemList(&types.ListContestProblemsRequest{Id: "c1"})
	if err != nil {
		t.Fatalf("problem list failed: %v", err)
	}
	if len(listResp.Data) != 1 || listResp.Data[0].ProblemId != 1001 {
		t.Fatalf("unexpected list response: %+v", listResp.Data)
	}

	_, err = logic.NewProblemUpdateLogic(context.Background(), ctx).ProblemUpdate(&types.UpdateContestProblemRequest{
		Id:        "c1",
		ProblemId: 1001,
		Order:     2,
		Score:     120,
		Visible:   false,
	})
	if err != nil {
		t.Fatalf("problem update failed: %v", err)
	}

	_, err = logic.NewProblemRemoveLogic(context.Background(), ctx).ProblemRemove(&types.RemoveContestProblemRequest{
		Id:        "c1",
		ProblemId: 1001,
	})
	if err != nil {
		t.Fatalf("problem remove failed: %v", err)
	}
}

func TestProblemUpdate_NotFound(t *testing.T) {
	contestStore := &fakeContestStoreRepo{
		getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
			return repository.ContestDetail{
				ContestID: contestID,
				Status:    "published",
				StartAt:   time.Now().Add(-time.Hour),
				EndAt:     time.Now().Add(time.Hour),
			}, nil
		},
	}
	ctx := newOpsServiceContext(contestStore, &fakeContestProblemStore{}, &fakeContestParticipantStore{})
	_, err := logic.NewProblemUpdateLogic(context.Background(), ctx).ProblemUpdate(&types.UpdateContestProblemRequest{
		Id:        "c1",
		ProblemId: 1,
		Order:     1,
		Score:     1,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if pkgerrors.GetCode(err) != pkgerrors.ProblemNotFound {
		t.Fatalf("unexpected code: %d", pkgerrors.GetCode(err))
	}
}

func TestRegisterLogic_StoreError(t *testing.T) {
	contestStore := &fakeContestStoreRepo{
		getFn: func(ctx context.Context, contestID string) (repository.ContestDetail, error) {
			return repository.ContestDetail{
				ContestID: contestID,
				Status:    "published",
				StartAt:   time.Now().Add(-time.Hour),
				EndAt:     time.Now().Add(time.Hour),
			}, nil
		},
	}
	participantStore := &fakeContestParticipantStore{
		findErr: errors.New("db down"),
	}
	ctx := newOpsServiceContext(contestStore, &fakeContestProblemStore{}, participantStore)
	_, err := logic.NewRegisterLogic(context.Background(), ctx).Register(&types.RegisterContestRequest{
		Id:     "c1",
		UserId: 100,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if pkgerrors.GetCode(err) != pkgerrors.DatabaseError {
		t.Fatalf("unexpected code: %d", pkgerrors.GetCode(err))
	}
}
