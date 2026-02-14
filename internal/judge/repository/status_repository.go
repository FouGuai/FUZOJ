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

// GetBatch returns statuses for multiple submission ids.
func (r *StatusRepository) GetBatch(ctx context.Context, submissionIDs []string) ([]model.JudgeStatusResponse, []string, error) {
	if len(submissionIDs) == 0 {
		return nil, nil, appErr.ValidationError("submission_ids", "required")
	}
	if r.cache == nil {
		return nil, nil, appErr.New(appErr.CacheError).WithMessage("cache client is not initialized")
	}
	keys := make([]string, 0, len(submissionIDs))
	for _, submissionID := range submissionIDs {
		if submissionID == "" {
			return nil, nil, appErr.ValidationError("submission_id", "required")
		}
		keys = append(keys, statusKeyPrefix+submissionID)
	}
	values, err := r.cache.MGet(ctx, keys...)
	if err != nil {
		return nil, nil, appErr.Wrapf(err, appErr.CacheError, "batch get status failed")
	}
	statuses := make([]model.JudgeStatusResponse, 0, len(submissionIDs))
	missing := make([]string, 0)
	for i, raw := range values {
		if raw == "" {
			missing = append(missing, submissionIDs[i])
			continue
		}
		var resp model.JudgeStatusResponse
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			return nil, nil, appErr.Wrapf(err, appErr.CacheError, "decode status failed")
		}
		statuses = append(statuses, resp)
	}
	if len(values) < len(submissionIDs) {
		missing = append(missing, submissionIDs[len(values):]...)
	}
	return statuses, missing, nil
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
