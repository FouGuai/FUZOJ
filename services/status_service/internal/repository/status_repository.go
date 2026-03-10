package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"fuzoj/internal/common/cache_helper"
	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statuscache"
	"fuzoj/services/status_service/internal/domain"
	"fuzoj/services/status_service/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"golang.org/x/sync/singleflight"
)

const (
	defaultStatusCacheTTL      = 30 * time.Minute
	defaultStatusCacheEmptyTTL = 5 * time.Minute
)

// StatusRepository handles status persistence.
type StatusRepository struct {
	cache            *redis.Redis
	submissionsModel model.SubmissionsModel
	ttl              time.Duration
	emptyTTL         time.Duration
	group            singleflight.Group
}

// NewStatusRepository creates a new repository.
func NewStatusRepository(cacheClient *redis.Redis, submissionsModel model.SubmissionsModel, ttl, emptyTTL time.Duration) *StatusRepository {
	if ttl <= 0 {
		ttl = defaultStatusCacheTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultStatusCacheEmptyTTL
	}
	return &StatusRepository{
		cache:            cacheClient,
		submissionsModel: submissionsModel,
		ttl:              ttl,
		emptyTTL:         emptyTTL,
	}
}

// Get returns status by submission id.
func (r *StatusRepository) Get(ctx context.Context, submissionID string) (domain.JudgeStatusPayload, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("get status start submission_id=%s", submissionID)
	if submissionID == "" {
		logger.Error("submission_id is required")
		return domain.JudgeStatusPayload{}, appErr.ValidationError("submission_id", "required")
	}
	if r.cache == nil {
		return r.getFinalStatusFromDB(ctx, submissionID)
	}

	cached, hit, err := statuscache.Get(ctx, r.cache, submissionID)
	if err != nil {
		logger.Errorf("get status cache failed: %v", err)
		return domain.JudgeStatusPayload{}, appErr.Wrapf(err, appErr.CacheError, "get status cache failed")
	}
	if hit {
		if cached == statuscache.NullValue {
			return domain.JudgeStatusPayload{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
		}
		status, err := unmarshalStatus(cached)
		if err == nil && status != nil {
			return *status, nil
		}
		logger.Errorf("decode status cache failed: %v", err)
	}

	value, err, _ := r.group.Do(submissionID, func() (any, error) {
		return r.getFinalStatusFromDB(ctx, submissionID)
	})
	if err != nil {
		if appErr.Is(err, appErr.NotFound) {
			_ = statuscache.Set(ctx, r.cache, submissionID, statuscache.NullValue, durationSeconds(cache_helper.JitterTTL(r.emptyTTL)))
			return domain.JudgeStatusPayload{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
		}
		return domain.JudgeStatusPayload{}, err
	}
	status, ok := value.(domain.JudgeStatusPayload)
	if !ok {
		return domain.JudgeStatusPayload{}, appErr.New(appErr.ServiceUnavailable).WithMessage("status payload type is invalid")
	}
	payload := mustMarshalStatus(statusSummary(status))
	if payload != "" {
		_ = statuscache.Set(ctx, r.cache, submissionID, payload, durationSeconds(cache_helper.JitterTTL(r.ttl)))
	}
	return statusSummary(status), nil
}

// GetFinalDetail returns final status payload from database directly.
func (r *StatusRepository) GetFinalDetail(ctx context.Context, submissionID string) (domain.JudgeStatusPayload, error) {
	if submissionID == "" {
		return domain.JudgeStatusPayload{}, appErr.ValidationError("submission_id", "required")
	}
	return r.getFinalStatusFromDB(ctx, submissionID)
}

func (r *StatusRepository) getFinalStatusFromDB(ctx context.Context, submissionID string) (domain.JudgeStatusPayload, error) {
	if r.submissionsModel == nil {
		return domain.JudgeStatusPayload{}, appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
	}
	payload, err := r.submissionsModel.FindFinalStatus(ctx, submissionID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return domain.JudgeStatusPayload{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
		}
		return domain.JudgeStatusPayload{}, appErr.Wrapf(err, appErr.DatabaseError, "get submission status failed")
	}
	status, err := unmarshalStatus(payload)
	if err != nil {
		return domain.JudgeStatusPayload{}, appErr.Wrapf(err, appErr.DatabaseError, "decode status failed")
	}
	return *status, nil
}

func mustMarshalStatus(status domain.JudgeStatusPayload) string {
	data, err := json.Marshal(status)
	if err != nil {
		return ""
	}
	return string(data)
}

func unmarshalStatus(data string) (*domain.JudgeStatusPayload, error) {
	var resp domain.JudgeStatusPayload
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func statusSummary(status domain.JudgeStatusPayload) domain.JudgeStatusPayload {
	summary := status
	summary.Compile = nil
	summary.Tests = nil
	return summary
}

func durationSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	seconds := int(ttl / time.Second)
	if seconds <= 0 {
		return 1
	}
	return seconds
}
