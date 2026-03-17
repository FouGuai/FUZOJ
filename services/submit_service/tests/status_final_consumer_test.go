package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"fuzoj/services/submit_service/internal/consumer"
	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/repository"
)

func TestStatusFinalConsumer_ExtractLogsOnly(t *testing.T) {
	_, redisClient := newTestRedis(t)
	var storedPayload string
	model := &fakeSubmissionsModel{
		updateFinalStatusFn: func(ctx context.Context, submissionID, payload string, finishedAt time.Time) (sql.Result, error) {
			storedPayload = payload
			return fakeSQLResult{rows: 1}, nil
		},
	}
	statusRepo := repository.NewStatusRepository(redisClient, model, time.Minute, time.Minute)
	logRepo := repository.NewSubmissionLogRepository(&fakeLogConn{}, redisClient, nil, "", "logs", 1024, time.Minute)
	consumerNoLogs := consumer.NewStatusFinalConsumer(statusRepo, (*repository.SubmissionLogRepository)(nil), nil, consumer.TimeoutConfig{DB: time.Second})
	consumerWithLogs := consumer.NewStatusFinalConsumer(statusRepo, logRepo, nil, consumer.TimeoutConfig{DB: time.Second})

	status := domain.JudgeStatusPayload{
		SubmissionID: "sub-1",
		Status:       domain.StatusFinished,
		Compile: &domain.CompileResult{
			OK:    false,
			Log:   "compile log",
			Error: "compile error",
		},
		Tests: []domain.TestcaseResult{
			{
				TestID:     "1",
				RuntimeLog: "runtime",
				CheckerLog: "checker",
				Stdout:     "out",
				Stderr:     "err",
			},
		},
		Timestamps: domain.Timestamps{ReceivedAt: time.Now().Unix(), FinishedAt: time.Now().Unix()},
	}
	event := domain.StatusEvent{Type: domain.StatusEventFinal, Status: status, CreatedAt: time.Now().Unix()}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event failed: %v", err)
	}

	if err := consumerWithLogs.Consume(context.Background(), "sub-1", string(payload)); err != nil {
		t.Fatalf("consume failed: %v", err)
	}
	if storedPayload != "" {
		t.Fatalf("expected final status payload not to be stored")
	}

	if err := consumerNoLogs.Consume(context.Background(), "sub-1", string(payload)); err != nil {
		t.Fatalf("consume without log repo failed: %v", err)
	}
}

func TestStatusFinalConsumer_MarkDispatchDone(t *testing.T) {
	model := &fakeSubmissionsModel{}
	statusRepo := repository.NewStatusRepository(nil, model, time.Minute, time.Minute)
	dispatchRepo := &fakeDispatchRepo{}
	handler := consumer.NewDispatchDoneHandler(dispatchRepo)
	c := consumer.NewStatusFinalConsumer(statusRepo, nil, []consumer.FinalStatusHandler{handler}, consumer.TimeoutConfig{DB: time.Second})

	event := domain.StatusEvent{
		Type: domain.StatusEventFinal,
		Status: domain.JudgeStatusPayload{
			SubmissionID: "sub-final-1",
			Status:       domain.StatusFinished,
			Timestamps:   domain.Timestamps{FinishedAt: time.Now().Unix()},
		},
		CreatedAt: time.Now().Unix(),
	}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event failed: %v", err)
	}
	if err := c.Consume(context.Background(), event.Status.SubmissionID, string(raw)); err != nil {
		t.Fatalf("consume failed: %v", err)
	}
	if len(dispatchRepo.markedDoneSubmission) != 1 {
		t.Fatalf("expected dispatch mark done once")
	}
	if dispatchRepo.markedDoneSubmission[0] != event.Status.SubmissionID {
		t.Fatalf("unexpected marked submission id: %s", dispatchRepo.markedDoneSubmission[0])
	}
}
