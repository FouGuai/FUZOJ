package judge_service

import (
	"context"
	"testing"
	"time"

	"fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/repository"
	"fuzoj/services/judge_service/internal/sandbox/result"
)

type fakeStatusPublisher struct {
	called int
	status pmodel.JudgeStatusResponse
	err    error
}

func (f *fakeStatusPublisher) PublishFinalStatus(ctx context.Context, status pmodel.JudgeStatusResponse) error {
	f.called++
	f.status = status
	return f.err
}

type fakeStatusBatcher struct {
	called int
	status pmodel.JudgeStatusResponse
	err    error
}

func (f *fakeStatusBatcher) Enqueue(ctx context.Context, status pmodel.JudgeStatusResponse) error {
	f.called++
	f.status = status
	return f.err
}

func TestStatusRepositorySaveEnqueueFinalStatus(t *testing.T) {
	t.Parallel()
	batcher := &fakeStatusBatcher{}
	repo := repository.NewStatusRepository(nil, nil, time.Minute, time.Minute, batcher)
	status := pmodel.JudgeStatusResponse{
		SubmissionID: "sub-1",
		Status:       result.StatusFinished,
	}
	if err := repo.Save(context.Background(), status); err != nil {
		t.Fatalf("save final status failed: %v", err)
	}
	if batcher.called != 1 {
		t.Fatalf("expected batcher called once, got %d", batcher.called)
	}
	if batcher.status.SubmissionID != status.SubmissionID {
		t.Fatalf("unexpected submission id: %s", batcher.status.SubmissionID)
	}
}

func TestStatusRepositorySaveSkipsNonFinalStatus(t *testing.T) {
	t.Parallel()
	batcher := &fakeStatusBatcher{}
	repo := repository.NewStatusRepository(nil, nil, time.Minute, time.Minute, batcher)
	status := pmodel.JudgeStatusResponse{
		SubmissionID: "sub-2",
		Status:       result.StatusRunning,
	}
	if err := repo.Save(context.Background(), status); err != nil {
		t.Fatalf("save non-final status failed: %v", err)
	}
	if batcher.called != 0 {
		t.Fatalf("expected batcher not called, got %d", batcher.called)
	}
}

func TestStatusRepositorySaveFinalStatusWithoutPublisher(t *testing.T) {
	t.Parallel()
	repo := repository.NewStatusRepository(nil, nil, time.Minute, time.Minute, nil)
	status := pmodel.JudgeStatusResponse{
		SubmissionID: "sub-3",
		Status:       result.StatusFinished,
	}
	if err := repo.Save(context.Background(), status); err != nil {
		t.Fatalf("save final status without publisher failed: %v", err)
	}
}
