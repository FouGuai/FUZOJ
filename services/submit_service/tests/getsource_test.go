package tests

import (
	"context"
	"net/http"
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/submit_service/internal/handler"
	"fuzoj/services/submit_service/internal/repository"
	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestGetSourceHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		repo := &fakeSubmissionRepo{
			getByIDFn: func(ctx context.Context, session sqlx.Session, submissionID string) (*repository.Submission, error) {
				return &repository.Submission{
					SubmissionID: submissionID,
					ProblemID:    10,
					UserID:       20,
					ContestID:    "contest",
					LanguageID:   "go",
					SourceCode:   "package main",
					CreatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				}, nil
			},
		}
		ctx := newTestServiceContext(defaultTestConfig(), repo, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		rr := doRequest(t, handler.GetSourceHandler(ctx), http.MethodGet, "/api/v1/submissions/sub-1/source", nil, nil, map[string]string{"id": "sub-1"})
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[types.GetSourceResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.Success) {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if resp.Data.SubmissionId != "sub-1" || resp.Data.ProblemId != 10 {
			t.Fatalf("unexpected data: %+v", resp.Data)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		_, redisClient := newTestRedis(t)
		model := &fakeSubmissionsModel{}
		statusRepo := repository.NewStatusRepository(redisClient, model, 5*time.Minute, time.Minute)
		ctx := newTestServiceContext(defaultTestConfig(), &fakeSubmissionRepo{}, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		rr := doRequest(t, handler.GetSourceHandler(ctx), http.MethodGet, "/api/v1/submissions//source", nil, nil, map[string]string{"id": ""})
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
		repo := &fakeSubmissionRepo{
			getByIDFn: func(ctx context.Context, session sqlx.Session, submissionID string) (*repository.Submission, error) {
				return nil, repository.ErrSubmissionNotFound
			},
		}
		ctx := newTestServiceContext(defaultTestConfig(), repo, statusRepo, &fakeStorage{}, redisClient, svc.TopicPushers{})
		rr := doRequest(t, handler.GetSourceHandler(ctx), http.MethodGet, "/api/v1/submissions/sub-miss/source", nil, nil, map[string]string{"id": "sub-miss"})
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status: %d", rr.Code)
		}
		resp := decodeJSON[errorResponse](t, rr.Body)
		if resp.Code != int(pkgerrors.SubmissionNotFound) {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
