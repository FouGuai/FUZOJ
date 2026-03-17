package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statuscache"
	"fuzoj/pkg/submit/statuspubsub"
	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/model"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	defaultStatusCacheTTL      = 30 * time.Minute
	defaultStatusCacheEmptyTTL = 5 * time.Minute
)

// StatusRepository handles status persistence.
type StatusRepository struct {
	redis            *redis.Redis
	pubsub           *red.Client
	submissionsModel model.SubmissionsModel
	ttl              time.Duration
	emptyTTL         time.Duration
}

// NewStatusRepository creates a new repository.
func NewStatusRepository(redisClient *redis.Redis, submissionsModel model.SubmissionsModel, ttl, emptyTTL time.Duration) *StatusRepository {
	if ttl <= 0 {
		ttl = defaultStatusCacheTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultStatusCacheEmptyTTL
	}
	return &StatusRepository{
		redis:            redisClient,
		submissionsModel: submissionsModel,
		ttl:              ttl,
		emptyTTL:         emptyTTL,
	}
}

// SetStatusPubSub sets status pubsub client for real-time notifications.
func (r *StatusRepository) SetStatusPubSub(client *red.Client) {
	if r == nil {
		return
	}
	r.pubsub = client
}

// Get returns status by submission id.
func (r *StatusRepository) Get(ctx context.Context, submissionID string) (domain.JudgeStatusPayload, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("get status start submission_id=%s", submissionID)
	if submissionID == "" {
		logger.Error("submission_id is required")
		return domain.JudgeStatusPayload{}, appErr.ValidationError("submission_id", "required")
	}
	if r.redis != nil {
		val, hit, err := statuscache.Get(ctx, r.redis, submissionID)
		if err != nil {
			logger.Errorf("get status cache failed: %v", err)
			return domain.JudgeStatusPayload{}, appErr.Wrapf(err, appErr.CacheError, "get status failed")
		}
		if hit {
			if val == statuscache.NullValue {
				return domain.JudgeStatusPayload{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
			}
			status, err := unmarshalStatus(val)
			if err != nil {
				logger.Errorf("decode status failed: %v", err)
				return domain.JudgeStatusPayload{}, appErr.Wrapf(err, appErr.CacheError, "decode status failed")
			}
			return *status, nil
		}
	}
	status, err := r.getFinalStatusFromDB(ctx, submissionID)
	if err != nil {
		return domain.JudgeStatusPayload{}, err
	}
	if r.redis != nil {
		if payload := mustMarshalStatus(statusSummary(status)); payload != "" {
			_ = statuscache.Set(ctx, r.redis, submissionID, payload, ttlSeconds(r.ttl))
		}
	}
	return status, nil
}

// GetBatch returns statuses for multiple submission ids.
func (r *StatusRepository) GetBatch(ctx context.Context, submissionIDs []string) ([]domain.JudgeStatusPayload, []string, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("get batch status start total=%d", len(submissionIDs))
	if len(submissionIDs) == 0 {
		logger.Error("submission_ids is required")
		return nil, nil, appErr.ValidationError("submission_ids", "required")
	}
	statuses := make([]domain.JudgeStatusPayload, 0, len(submissionIDs))
	missing := make([]string, 0)
	if r.redis != nil {
		keys := make([]string, 0, len(submissionIDs))
		for _, submissionID := range submissionIDs {
			if submissionID == "" {
				logger.Error("submission_id is required")
				return nil, nil, appErr.ValidationError("submission_id", "required")
			}
			keys = append(keys, statuscache.PrimaryKey(submissionID))
		}
		values, err := r.redis.MgetCtx(ctx, keys...)
		if err != nil {
			logger.Errorf("batch get status cache failed: %v", err)
			return nil, nil, appErr.Wrapf(err, appErr.CacheError, "batch get status failed")
		}
		for i, raw := range values {
			if raw == "" {
				missing = append(missing, submissionIDs[i])
				continue
			}
			if raw == statuscache.NullValue {
				continue
			}
			status, err := unmarshalStatus(raw)
			if err != nil {
				logger.Errorf("decode status failed: %v", err)
				return nil, nil, appErr.Wrapf(err, appErr.CacheError, "decode status failed")
			}
			statuses = append(statuses, *status)
		}
		if len(values) < len(submissionIDs) {
			missing = append(missing, submissionIDs[len(values):]...)
		}
	} else {
		missing = append(missing, submissionIDs...)
	}

	if len(missing) == 0 {
		return statuses, missing, nil
	}

	dbStatuses, dbMissing, err := r.getFinalStatusBatchFromDB(ctx, missing)
	if err != nil {
		logger.Errorf("batch get final status from db failed: %v", err)
		return nil, nil, err
	}
	if len(dbStatuses) > 0 {
		for _, st := range dbStatuses {
			statuses = append(statuses, statusSummary(st))
		}
		if r.redis != nil {
			for _, st := range dbStatuses {
				if payload := mustMarshalStatus(statusSummary(st)); payload != "" {
					_ = statuscache.Set(ctx, r.redis, st.SubmissionID, payload, ttlSeconds(r.ttl))
				}
			}
		}
	}
	if r.redis != nil && len(dbMissing) > 0 {
		for _, id := range dbMissing {
			if id == "" {
				continue
			}
			_ = statuscache.Set(ctx, r.redis, id, statuscache.NullValue, ttlSeconds(r.emptyTTL))
		}
	}
	return statuses, dbMissing, nil
}

// GetFinalDetail returns final status payload from database directly.
func (r *StatusRepository) GetFinalDetail(ctx context.Context, submissionID string) (domain.JudgeStatusPayload, error) {
	if submissionID == "" {
		return domain.JudgeStatusPayload{}, appErr.ValidationError("submission_id", "required")
	}
	return r.getFinalStatusFromDB(ctx, submissionID)
}

// Save persists status.
func (r *StatusRepository) Save(ctx context.Context, status domain.JudgeStatusPayload) error {
	logger := logx.WithContext(ctx)
	logger.Infof("save status start submission_id=%s status=%s", status.SubmissionID, status.Status)
	if status.SubmissionID == "" {
		logger.Error("submission_id is required")
		return appErr.ValidationError("submission_id", "required")
	}
	if r.redis == nil {
		return nil
	}
	data, err := json.Marshal(statusSummary(status))
	if err != nil {
		logger.Errorf("marshal status failed: %v", err)
		return fmt.Errorf("marshal status failed: %w", err)
	}
	if err := statuscache.Set(ctx, r.redis, status.SubmissionID, string(data), ttlSeconds(r.ttl)); err != nil {
		logger.Errorf("store status failed: %v", err)
		return appErr.Wrapf(err, appErr.CacheError, "store status failed")
	}
	if err := statuspubsub.Publish(ctx, r.pubsub, status.SubmissionID); err != nil {
		logger.Errorf("publish status update failed: %v", err)
	}
	return nil
}

// PersistFinalStatus stores final status into the database.
func (r *StatusRepository) PersistFinalStatus(ctx context.Context, status domain.JudgeStatusPayload) error {
	logger := logx.WithContext(ctx)
	logger.Infof("persist final status start submission_id=%s", status.SubmissionID)
	if status.SubmissionID == "" {
		logger.Error("submission_id is required")
		return appErr.ValidationError("submission_id", "required")
	}
	if !isFinalStatus(status.Status) {
		logger.Error("status must be final")
		return appErr.ValidationError("status", "final_required")
	}
	return r.storeFinalStatus(ctx, status)
}

func (r *StatusRepository) storeFinalStatus(ctx context.Context, status domain.JudgeStatusPayload) error {
	logger := logx.WithContext(ctx)
	payload, err := json.Marshal(status)
	if err != nil {
		logger.Errorf("marshal final status failed: %v", err)
		return fmt.Errorf("marshal final status failed: %w", err)
	}
	finishedAt := time.Now()
	if status.Timestamps.FinishedAt > 0 {
		finishedAt = time.Unix(status.Timestamps.FinishedAt, 0)
	}
	res, err := r.submissionsModel.UpdateFinalStatus(ctx, status.SubmissionID, string(payload), finishedAt)
	if err != nil {
		logger.Errorf("store final status failed: %v", err)
		return appErr.Wrapf(err, appErr.DatabaseError, "store final status failed")
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		logger.Error("submission not found")
		return appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
	}
	return nil
}

func (r *StatusRepository) getFinalStatusFromDB(ctx context.Context, submissionID string) (domain.JudgeStatusPayload, error) {
	payload, err := r.submissionsModel.FindFinalStatus(ctx, submissionID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			if r.redis != nil {
				_ = statuscache.Set(ctx, r.redis, submissionID, statuscache.NullValue, ttlSeconds(r.emptyTTL))
			}
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

func (r *StatusRepository) getFinalStatusBatchFromDB(ctx context.Context, submissionIDs []string) ([]domain.JudgeStatusPayload, []string, error) {
	rows, err := r.submissionsModel.FindFinalStatusBatch(ctx, submissionIDs)
	if err != nil {
		return nil, nil, appErr.Wrapf(err, appErr.DatabaseError, "batch get submission status failed")
	}
	statuses := make([]domain.JudgeStatusPayload, 0, len(rows))
	found := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		status, err := unmarshalStatus(row.FinalStatus)
		if err != nil {
			return nil, nil, appErr.Wrapf(err, appErr.DatabaseError, "decode status failed")
		}
		statuses = append(statuses, *status)
		found[row.SubmissionID] = struct{}{}
	}
	missing := make([]string, 0)
	for _, id := range submissionIDs {
		if id == "" {
			continue
		}
		if _, ok := found[id]; !ok {
			missing = append(missing, id)
		}
	}
	return statuses, missing, nil
}

func isFinalStatus(status string) bool {
	return status == domain.StatusFinished || status == domain.StatusFailed
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

func ttlSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}
