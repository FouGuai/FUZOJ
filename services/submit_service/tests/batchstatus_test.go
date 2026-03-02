package tests

import (
	"context"
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

func TestBatchStatusHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		status1 := domain.JudgeStatusPayload{
			SubmissionID: "sub-1",
			Status:       domain.StatusFinished,
			Timestamps:   domain.Timestamps{ReceivedAt: time.Now().Unix(), FinishedAt: time.Now().Unix()},
			Progress:     domain.Progress{TotalTests: 1, DoneTests: 1},
		}
		status2 := domain.JudgeStatusPayload{
			SubmissionID: "sub-2",
			Status:       domain.StatusPending,
			Timestamps:   domain.Timestamps{ReceivedAt: time.Now().Unix()},
			Progress:     domain.Progress{TotalTests: 0, DoneTests: 0},
		}
		data2, err := json.Marshal(status2)
		if err != nil {
			t.Fatalf("marshal payload failed: %v", err)
		}
		model := &fakeSubmissionsModel{finalStatus: map[string]string{"sub-2": string(data2)}}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		if err := statusRepo.Save(context.Background(), status1); err != nil {
			t.Fatalf("store status failed: %v", err)
		}
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, nil, &fakeStorage{}, redisClient, svc.TopicPushers{})
		req := types.BatchStatusRequest{SubmissionIds: []string{"sub-1", "sub-2", "sub-3"}}
		rr := doRequest(t, handler.BatchStatusHandler(ctx), http.MethodPost, "/api/v1/submissions/batch_status", req, nil, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.BatchStatusResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if len(resp.Data.Items) != 2 || len(resp.Data.Missing) != 1 {
			t.Fatalf("unexpected data: %+v", resp.Data)
		}
		found := map[string]struct{}{}
		for _, item := range resp.Data.Items {
			found[item.SubmissionId] = struct{}{}
		}
		if _, ok := found["sub-1"]; !ok {
			t.Fatalf("missing status for sub-1")
		}
		if _, ok := found["sub-2"]; !ok {
			t.Fatalf("missing status for sub-2")
		}
		if resp.Data.Missing[0] != "sub-3" {
			t.Fatalf("unexpected missing: %+v", resp.Data.Missing)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, nil, &fakeStorage{}, redisClient, svc.TopicPushers{})
		rr := doRequest(t, handler.BatchStatusHandler(ctx), http.MethodPost, "/api/v1/submissions/batch_status", "{", nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, nil, &fakeStorage{}, redisClient, svc.TopicPushers{})
		req := types.BatchStatusRequest{SubmissionIds: []string{}}
		rr := doRequest(t, handler.BatchStatusHandler(ctx), http.MethodPost, "/api/v1/submissions/batch_status", req, nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ValidationFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("too many", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		cfg := defaultTestConfig()
		cfg.Submit.BatchLimit = 1
		ctx := newTestServiceContext(cfg, &fakeSubmissionRepo{}, statusRepo, nil, &fakeStorage{}, redisClient, svc.TopicPushers{})
		req := types.BatchStatusRequest{SubmissionIds: []string{"sub-1", "sub-2"}}
		rr := doRequest(t, handler.BatchStatusHandler(ctx), http.MethodPost, "/api/v1/submissions/batch_status", req, nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ValidationFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("contains empty id", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, nil, &fakeStorage{}, redisClient, svc.TopicPushers{})
		req := types.BatchStatusRequest{SubmissionIds: []string{"sub-1", ""}}
		rr := doRequest(t, handler.BatchStatusHandler(ctx), http.MethodPost, "/api/v1/submissions/batch_status", req, nil, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.ValidationFailed) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
