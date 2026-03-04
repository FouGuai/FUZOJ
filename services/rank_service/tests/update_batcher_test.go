package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_service/internal/consumer"
	"fuzoj/services/rank_service/internal/pmodel"
)

type fakeUpdateRepo struct {
	mu        sync.Mutex
	calls     int
	failUntil int
	lastBatch []pmodel.RankUpdateEvent
	done      chan struct{}
}

func newFakeUpdateRepo(failUntil int) *fakeUpdateRepo {
	return &fakeUpdateRepo{
		failUntil: failUntil,
		done:      make(chan struct{}),
	}
}

func (r *fakeUpdateRepo) ApplyUpdates(ctx context.Context, events []pmodel.RankUpdateEvent) error {
	r.mu.Lock()
	r.calls++
	r.lastBatch = append([]pmodel.RankUpdateEvent(nil), events...)
	call := r.calls
	r.mu.Unlock()
	if call <= r.failUntil {
		return appErr.New(appErr.DatabaseError).WithMessage("forced failure")
	}
	select {
	case <-r.done:
	default:
		close(r.done)
	}
	return nil
}

func (r *fakeUpdateRepo) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func TestUpdateBatcher_AddBlocksAndTimeout(t *testing.T) {
	repo := newFakeUpdateRepo(0)
	batcher := consumer.NewUpdateBatcher(repo, nil, 1, time.Hour, time.Second)
	for i := 0; i < 4; i++ {
		if err := batcher.Add(context.Background(), pmodel.RankUpdateEvent{
			ContestID: "c1",
			MemberID:  "m1",
			Version:   "1",
		}); err != nil {
			t.Fatalf("unexpected add error: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := batcher.Add(ctx, pmodel.RankUpdateEvent{
		ContestID: "c1",
		MemberID:  "m1",
		Version:   "2",
	})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if appErr.GetCode(err) != appErr.Timeout {
		t.Fatalf("expected timeout error code, got %v", err)
	}
}

func TestUpdateBatcher_RetryOnApplyFailure(t *testing.T) {
	repo := newFakeUpdateRepo(2)
	batcher := consumer.NewUpdateBatcher(repo, nil, 2, 20*time.Millisecond, 200*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go batcher.Start(ctx)
	defer batcher.Stop()

	for i := 0; i < 2; i++ {
		if err := batcher.Add(context.Background(), pmodel.RankUpdateEvent{
			ContestID: "c1",
			MemberID:  "m1",
			Version:   "10",
		}); err != nil {
			t.Fatalf("unexpected add error: %v", err)
		}
	}

	select {
	case <-repo.done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected batch to eventually succeed")
	}
	if repo.Calls() < 3 {
		t.Fatalf("expected retries, got %d calls", repo.Calls())
	}
}
