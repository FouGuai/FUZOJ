package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/judge/model"
	appErr "fuzoj/pkg/errors"
)

const statusKeyPrefix = "judge:status:"

// StatusRepository handles status persistence.
type StatusRepository struct {
	cache cache.Cache
	TTL   time.Duration
}

// NewStatusRepository creates a new repository.
func NewStatusRepository(cacheClient cache.Cache, ttl time.Duration) *StatusRepository {
	return &StatusRepository{cache: cacheClient, TTL: ttl}
}

// Get returns status by submission id.
func (r *StatusRepository) Get(ctx context.Context, submissionID string) (model.JudgeStatusResponse, error) {
	if submissionID == "" {
		return model.JudgeStatusResponse{}, appErr.ValidationError("submission_id", "required")
	}
	if r.cache == nil {
		return model.JudgeStatusResponse{}, appErr.New(appErr.CacheError).WithMessage("cache client is not initialized")
	}
	val, err := r.cache.Get(ctx, statusKeyPrefix+submissionID)
	if err != nil || val == "" {
		return model.JudgeStatusResponse{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
	}
	var resp model.JudgeStatusResponse
	if err := json.Unmarshal([]byte(val), &resp); err != nil {
		return model.JudgeStatusResponse{}, appErr.Wrapf(err, appErr.CacheError, "decode status failed")
	}
	return resp, nil
}

// Save persists status.
func (r *StatusRepository) Save(ctx context.Context, status model.JudgeStatusResponse) error {
	if status.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if r.cache == nil {
		return appErr.New(appErr.CacheError).WithMessage("cache client is not initialized")
	}
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal status failed: %w", err)
	}
	if err := r.cache.Set(ctx, statusKeyPrefix+status.SubmissionID, string(data), r.TTL); err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "store status failed")
	}
	return nil
}
