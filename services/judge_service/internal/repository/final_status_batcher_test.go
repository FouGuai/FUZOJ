package repository

import (
	"context"
	"testing"
	"time"

	"fuzoj/services/judge_service/internal/model"
	"fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/sandbox/result"
)

type fakeBatchSubmissionsModel struct {
	model.SubmissionsModel
	calls int
}

func (f *fakeBatchSubmissionsModel) UpdateFinalStatusBatch(ctx context.Context, records []model.FinalStatusRecord) error {
	f.calls++
	return nil
}

type fakeBatchPublisher struct {
	failCount int
	calls     int
}

func (f *fakeBatchPublisher) PublishFinalStatus(ctx context.Context, status pmodel.JudgeStatusResponse) error {
	f.calls++
	if f.calls <= f.failCount {
		return context.DeadlineExceeded
	}
	return nil
}

func TestFinalStatusBatcher_RequeueOnPublishFailure(t *testing.T) {
	t.Parallel()

	modelStub := &fakeBatchSubmissionsModel{}
	publisherStub := &fakeBatchPublisher{failCount: 1}
	batcher := NewFinalStatusBatcher(modelStub, publisherStub, 10, time.Hour, time.Second)

	item := pmodel.JudgeStatusResponse{
		SubmissionID: "sub-1",
		Status:       result.StatusFinished,
	}
	if err := batcher.Enqueue(context.Background(), item); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	batcher.flush(context.Background())
	if publisherStub.calls != 1 {
		t.Fatalf("expected publish called once, got %d", publisherStub.calls)
	}
	if len(batcher.buffer) != 1 {
		t.Fatalf("expected failed publish requeued, buffer=%d", len(batcher.buffer))
	}

	batcher.flush(context.Background())
	if publisherStub.calls != 2 {
		t.Fatalf("expected publish retried once, got %d", publisherStub.calls)
	}
	if len(batcher.buffer) != 0 {
		t.Fatalf("expected buffer drained after successful retry, buffer=%d", len(batcher.buffer))
	}
	if modelStub.calls != 2 {
		t.Fatalf("expected db batch called twice, got %d", modelStub.calls)
	}
}

func TestFinalStatusBatcher_EnqueueSignalsOnFirstItem(t *testing.T) {
	t.Parallel()

	modelStub := &fakeBatchSubmissionsModel{}
	batcher := NewFinalStatusBatcher(modelStub, nil, 10, time.Hour, time.Second)

	item := pmodel.JudgeStatusResponse{
		SubmissionID: "sub-1",
		Status:       result.StatusFinished,
	}
	if err := batcher.Enqueue(context.Background(), item); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	select {
	case <-batcher.signalCh:
	default:
		t.Fatal("expected signal on first enqueue")
	}
}
