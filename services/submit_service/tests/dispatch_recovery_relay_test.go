package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"fuzoj/services/submit_service/internal/consumer"
	"fuzoj/services/submit_service/internal/repository"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type fakeDispatchChecker struct {
	finalized bool
	err       error
}

func (f *fakeDispatchChecker) HasFinalStatus(ctx context.Context, submissionID string) (bool, error) {
	return f.finalized, f.err
}

type fakeDispatchRouter struct {
	name   string
	pusher consumer.MessagePusher
}

func (f *fakeDispatchRouter) ResolveDispatchTarget(record repository.SubmissionDispatchRecord) (string, consumer.MessagePusher) {
	return f.name, f.pusher
}

func TestDispatchRecoveryRelayRepublish(t *testing.T) {
	var mu sync.Mutex
	claimed := false
	markPublished := 0
	repo := &fakeDispatchRepo{
		claimDueFn: func(ctx context.Context, now time.Time, ownerID string, leaseDuration time.Duration, limit int) ([]repository.SubmissionDispatchRecord, error) {
			mu.Lock()
			defer mu.Unlock()
			if claimed {
				return nil, nil
			}
			claimed = true
			return []repository.SubmissionDispatchRecord{
				{
					ID:           1,
					SubmissionID: "sub-1",
					Scene:        "practice",
					Payload:      `{"submission_id":"sub-1"}`,
					RetryCount:   0,
				},
			}, nil
		},
		markPublishedFn: func(ctx context.Context, id int64, ownerID string, nextRetryAt time.Time) error {
			mu.Lock()
			defer mu.Unlock()
			markPublished++
			return nil
		},
	}
	pusher := &fakePusher{}
	relay := consumer.NewDispatchRecoveryRelay(
		repo,
		&fakeDispatchChecker{},
		&fakeDispatchRouter{name: "judge.level1", pusher: pusher},
		nil,
		consumer.DispatchRecoveryOptions{
			Enabled:       true,
			ScanInterval:  20 * time.Millisecond,
			ClaimBatch:    10,
			WorkerCount:   1,
			LeaseDuration: time.Second,
			TimeoutAfter:  time.Minute,
			DBTimeout:     time.Second,
			MQTimeout:     time.Second,
		},
	)
	relay.Start()
	defer relay.Stop()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		mp := markPublished
		mu.Unlock()
		if mp > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if markPublished != 1 {
		t.Fatalf("expected markPublished once, got %d", markPublished)
	}
	if len(pusher.keys) != 1 || pusher.keys[0] != "sub-1" {
		t.Fatalf("expected republish for sub-1, keys=%v", pusher.keys)
	}
}

func TestDispatchRecoveryRelayMarkDoneWhenFinalized(t *testing.T) {
	var markDone int
	repo := &fakeDispatchRepo{
		claimDueFn: func(ctx context.Context, now time.Time, ownerID string, leaseDuration time.Duration, limit int) ([]repository.SubmissionDispatchRecord, error) {
			if markDone > 0 {
				return nil, nil
			}
			return []repository.SubmissionDispatchRecord{
				{ID: 1, SubmissionID: "sub-done", Scene: "practice", Payload: "{}"},
			}, nil
		},
		markDoneFn: func(ctx context.Context, session sqlx.Session, submissionID string) error {
			markDone++
			return nil
		},
	}
	relay := consumer.NewDispatchRecoveryRelay(
		repo,
		&fakeDispatchChecker{finalized: true},
		&fakeDispatchRouter{name: "judge.level1", pusher: &fakePusher{}},
		nil,
		consumer.DispatchRecoveryOptions{
			Enabled:       true,
			ScanInterval:  20 * time.Millisecond,
			ClaimBatch:    10,
			WorkerCount:   1,
			LeaseDuration: time.Second,
			TimeoutAfter:  time.Minute,
			DBTimeout:     time.Second,
			MQTimeout:     time.Second,
		},
	)
	relay.Start()
	defer relay.Stop()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if markDone > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if markDone != 1 {
		t.Fatalf("expected markDone once, got %d", markDone)
	}
}
