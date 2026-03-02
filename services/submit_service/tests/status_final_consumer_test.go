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

func TestStatusFinalConsumer_ExtractLogsAndPersistSummary(t *testing.T) {
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
	if storedPayload == "" {
		t.Fatalf("expected final status payload to be stored")
	}
	var stored domain.JudgeStatusPayload
	if err := json.Unmarshal([]byte(storedPayload), &stored); err != nil {
		t.Fatalf("decode stored payload failed: %v", err)
	}
	if stored.Compile != nil && (stored.Compile.Log != "" || stored.Compile.Error != "") {
		t.Fatalf("expected compile logs to be stripped")
	}
	if len(stored.Tests) > 0 {
		if stored.Tests[0].RuntimeLog != "" || stored.Tests[0].CheckerLog != "" || stored.Tests[0].Stdout != "" || stored.Tests[0].Stderr != "" {
			t.Fatalf("expected testcase logs to be stripped")
		}
	}

	if err := consumerNoLogs.Consume(context.Background(), "sub-1", string(payload)); err != nil {
		t.Fatalf("consume without log repo failed: %v", err)
	}
	statusSummary, err := statusRepo.Get(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("status repo get failed: %v", err)
	}
	if statusSummary.Compile != nil || len(statusSummary.Tests) != 0 {
		t.Fatalf("expected summary cache without compile/tests")
	}
}
