package tests

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/handler"
	"fuzoj/services/submit_service/internal/repository"
	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"
)

func TestGetStatusHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		payload := domain.JudgeStatusPayload{
			SubmissionID: "sub-1",
			Status:       domain.StatusFinished,
			Verdict:      "AC",
			Score:        100,
			Language:     "go",
			Timestamps:   domain.Timestamps{ReceivedAt: time.Now().Unix(), FinishedAt: time.Now().Unix()},
			Progress:     domain.Progress{TotalTests: 1, DoneTests: 1},
		}
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload failed: %v", err)
		}
		model := &fakeSubmissionsModel{finalStatus: map[string]string{"sub-1": string(data)}}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		rr := doRequest(t, handler.GetStatusHandler(ctx), http.MethodGet, "/api/v1/submissions/sub-1", nil, nil, map[string]string{"id": "sub-1"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.GetStatusResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if resp.Data.SubmissionId != payload.SubmissionID || resp.Data.Status != payload.Status {
			t.Fatalf("unexpected data: %+v", resp.Data)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		rr := doRequest(t, handler.GetStatusHandler(ctx), http.MethodGet, "/api/v1/submissions/", nil, nil, map[string]string{"id": ""})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ValidationFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		rr := doRequest(t, handler.GetStatusHandler(ctx), http.MethodGet, "/api/v1/submissions/sub-miss", nil, nil, map[string]string{"id": "sub-miss"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.NotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
