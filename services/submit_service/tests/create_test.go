package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/submit_service/internal/config"
	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/handler"
	"fuzoj/services/submit_service/internal/repository"
	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"

	"fuzoj/internal/common/storage"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestCreateHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		var created *repository.Submission
		storageCalls := 0
		pusher := &fakePusher{}
		storageClient := &fakeStorage{
			putObjectFn: func(ctx context.Context, bucket, objectKey string, reader storage.ObjectReader, sizeBytes int64, contentType string) error {
				storageCalls++
				if !strings.HasSuffix(objectKey, "/source.code") {
					return pkgerrors.New(pkgerrors.SubmissionCreateFailed).WithMessage("invalid object key")
				}
				return nil
			},
		}
		repo := &fakeSubmissionRepo{
			createFn: func(ctx context.Context, session sqlx.Session, submission *repository.Submission) error {
				created = submission
				return nil
			},
		}
		cfg := defaultTestConfig()
		ctx := newTestServiceContext(cfg, repo, statusRepo, storageClient, redisClient, svc.TopicPushers{Level1: pusher})
		req := types.CreateSubmissionRequest{
			ProblemId:  100,
			UserId:     200,
			LanguageId: "go",
			SourceCode: "package main",
			ContestId:  "",
			Scene:      "practice",
			ExtraCompileFlags: []string{},
		}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/submissions", req, map[string]string{"Idempotency-Key": "test-idem"}, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.CreateSubmissionResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) || resp.Data.SubmissionId == "" {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if resp.Data.Status != domain.StatusPending {
			t.Fatalf("unexpected status: %s", resp.Data.Status)
		}
		if resp.Data.ReceivedAt == 0 {
			t.Fatalf("expected received_at to be set")
		}
		if created == nil || created.SubmissionID != resp.Data.SubmissionId {
			t.Fatalf("submission not created")
		}
		if storageCalls != 1 {
			t.Fatalf("unexpected storage calls: %d", storageCalls)
		}
		if len(pusher.keys) != 1 {
			t.Fatalf("expected pusher to be called")
		}
		if pusher.keys[0] != resp.Data.SubmissionId {
			t.Fatalf("unexpected pusher key: %s", pusher.keys[0])
		}
		var message domain.JudgeMessage
		if err := json.Unmarshal([]byte(pusher.values[0]), &message); err != nil {
			t.Fatalf("decode judge message failed: %v", err)
		}
		if message.SubmissionID != resp.Data.SubmissionId || message.ProblemID != req.ProblemId {
			t.Fatalf("unexpected judge message: %+v", message)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/submissions", "{", map[string]string{"Idempotency-Key": "test-idem"}, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.InvalidParams) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		cases := []struct {
			name string
			req  types.CreateSubmissionRequest
		}{
			{
				name: "missing problem",
				req: types.CreateSubmissionRequest{UserId: 1, LanguageId: "go", SourceCode: "code", ContestId: "", Scene: "practice", ExtraCompileFlags: []string{}},
			},
			{
				name: "missing user",
				req: types.CreateSubmissionRequest{ProblemId: 1, LanguageId: "go", SourceCode: "code", ContestId: "", Scene: "practice", ExtraCompileFlags: []string{}},
			},
			{
				name: "missing language",
				req: types.CreateSubmissionRequest{ProblemId: 1, UserId: 1, SourceCode: "code", ContestId: "", Scene: "practice", ExtraCompileFlags: []string{}},
			},
			{
				name: "missing source",
				req: types.CreateSubmissionRequest{ProblemId: 1, UserId: 1, LanguageId: "go", ContestId: "", Scene: "practice", ExtraCompileFlags: []string{}},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, redisClient := newTestRedis(t)
				model := &fakeSubmissionsModel{}
				statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
				ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
				rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/submissions", tc.req, map[string]string{"Idempotency-Key": "test-idem"}, nil)
				if rr.Code != http.StatusBadRequest {
					t.Fatalf("unexpected status: %d", rr.Code)
				}
				resp := decodeJSON[errorResponse](t, rr.Body)
				if resp.Code != int(pkgerrors.ValidationFailed) {
					t.Fatalf("unexpected response: %+v", resp)
				}
			})
		}
	})

	t.Run("code too large", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		cfg := defaultTestConfig()
		cfg.Submit.MaxCodeBytes = 1
		ctx := newTestServiceContext(cfg, &fakeSubmissionRepo{}, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		req := types.CreateSubmissionRequest{ProblemId: 1, UserId: 1, LanguageId: "go", SourceCode: "xx", ContestId: "", Scene: "practice", ExtraCompileFlags: []string{}}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/submissions", req, map[string]string{"Idempotency-Key": "test-idem"}, nil)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.CodeTooLarge) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("rate limit", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		cfg := defaultTestConfig()
		cfg.Submit.RateLimit = config.RateLimitConfig{UserMax: 1, IPMax: 0, Window: time.Minute}
		ctx := newTestServiceContext(cfg, &fakeSubmissionRepo{}, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		req := types.CreateSubmissionRequest{ProblemId: 1, UserId: 10, LanguageId: "go", SourceCode: "code", ContestId: "", Scene: "practice", ExtraCompileFlags: []string{}}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/submissions", req, map[string]string{"Idempotency-Key": "test-idem"}, nil)
		if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		rr = doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/submissions", req, map[string]string{"Idempotency-Key": "test-idem"}, nil)
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.SubmitTooFrequently) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("idempotency hit", func(t *testing.T) {
		mr, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		existingID := "existing-1"
		status := domain.JudgeStatusPayload{
			SubmissionID: existingID,
			Status:       domain.StatusPending,
			Timestamps:   domain.Timestamps{ReceivedAt: time.Now().Unix()},
		}
		if err := statusRepo.Save(context.Background(), status); err != nil {
			t.Fatalf("store status failed: %v", err)
		}
		if err := mr.Set("submit:idempotency:idem-key", existingID); err != nil {
			t.Fatalf("set idempotency key failed: %v", err)
		}
		calls := 0
		storageClient := &fakeStorage{putObjectFn: func(ctx context.Context, bucket, objectKey string, reader storage.ObjectReader, sizeBytes int64, contentType string) error {
			calls++
			return nil
		}}
		repo := &fakeSubmissionRepo{createFn: func(ctx context.Context, session sqlx.Session, submission *repository.Submission) error {
			calls++
			return nil
		}}
		pusher := &fakePusher{pushFn: func(ctx context.Context, key, value string) error {
			calls++
			return nil
		}}
		ctx := newTestServiceContext(defaultTestConfig(), repo, statusRepo, storageClient, redisClient, svc.TopicPushers{Level1: pusher})
		req := types.CreateSubmissionRequest{ProblemId: 1, UserId: 2, LanguageId: "go", SourceCode: "code", ContestId: "", Scene: "practice", ExtraCompileFlags: []string{}, IdempotencyKey: "idem-key"}
		rr := doRequest(t, handler.CreateHandler(ctx), http.MethodPost, "/api/v1/submissions", req, map[string]string{"Idempotency-Key": "idem-key"}, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.CreateSubmissionResponse](t, rr.Body)
		if resp.Data.SubmissionId != existingID {
			t.Fatalf("unexpected submission id: %s", resp.Data.SubmissionId)
		}
		if calls != 0 {
			t.Fatalf("unexpected side effects: %d", calls)
		}
	})
}
