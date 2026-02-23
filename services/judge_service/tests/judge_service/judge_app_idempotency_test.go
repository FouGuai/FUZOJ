package judge_service_test

import (
	"context"
	"reflect"
	"testing"
	"time"
	"unsafe"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/logic/judge_app"
	"fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/repository"
	"fuzoj/services/judge_service/internal/sandbox/result"
)

func TestJudgeAppHandleMessageIdempotentAheadStatus(t *testing.T) {
	t.Parallel()

	cache := newFakeCache()
	statusRepo := repository.NewStatusRepository(cache, nil, time.Minute, time.Minute, nil)
	// Store a running status in cache so it is ahead of pending.
	if err := statusRepo.Save(context.Background(), pmodel.JudgeStatusResponse{
		SubmissionID: "sub-1",
		Status:       result.StatusRunning,
	}); err != nil {
		t.Fatalf("seed status failed: %v", err)
	}

	app := &judge_app.JudgeApp{}
	field := reflect.ValueOf(app).Elem().FieldByName("statusRepo")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(statusRepo))
	payload := pmodel.JudgeMessage{
		SubmissionID: "sub-1",
		ProblemID:    1,
		LanguageID:   "go",
		SourceKey:    "src",
	}
	if err := app.HandleMessage(context.Background(), payload); err == nil || !appErr.Is(err, appErr.InvalidParams) {
		t.Fatalf("expected InvalidParams for ahead status, got %v", err)
	}
}
