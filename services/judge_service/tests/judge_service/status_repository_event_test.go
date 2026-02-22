package judge_service_test

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

func TestStatusRepositorySavePublishesFinalStatus(t *testing.T) {
	t.Parallel()
	pub := &fakeStatusPublisher{}
	repo := repository.NewStatusRepository(nil, nil, time.Minute, time.Minute, pub)
	status := pmodel.JudgeStatusResponse{
		SubmissionID: "sub-1",
		Status:       result.StatusFinished,
	}
	if err := repo.Save(context.Background(), status); err != nil {
		t.Fatalf("save final status failed: %v", err)
	}
	if pub.called != 1 {
		t.Fatalf("expected publisher called once, got %d", pub.called)
	}
	if pub.status.SubmissionID != status.SubmissionID {
		t.Fatalf("unexpected submission id: %s", pub.status.SubmissionID)
	}
}

func TestStatusRepositorySaveSkipsNonFinalStatus(t *testing.T) {
	t.Parallel()
	pub := &fakeStatusPublisher{}
	repo := repository.NewStatusRepository(nil, nil, time.Minute, time.Minute, pub)
	status := pmodel.JudgeStatusResponse{
		SubmissionID: "sub-2",
		Status:       result.StatusRunning,
	}
	if err := repo.Save(context.Background(), status); err != nil {
		t.Fatalf("save non-final status failed: %v", err)
	}
	if pub.called != 0 {
		t.Fatalf("expected publisher not called, got %d", pub.called)
	}
}

func TestStatusRepositorySaveFinalStatusRequiresPublisher(t *testing.T) {
	t.Parallel()
	repo := repository.NewStatusRepository(nil, nil, time.Minute, time.Minute, nil)
	status := pmodel.JudgeStatusResponse{
		SubmissionID: "sub-3",
		Status:       result.StatusFinished,
	}
	if err := repo.Save(context.Background(), status); err == nil {
		t.Fatalf("expected error when publisher is nil")
	}
}
