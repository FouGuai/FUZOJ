package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cachex "fuzoj/internal/common/cache"
	dbmodel "fuzoj/judge_service/internal/model"
	"fuzoj/judge_service/internal/pmodel"
	"fuzoj/judge_service/internal/sandbox/result"
	appErr "fuzoj/pkg/errors"

	"github.com/zeromicro/go-zero/core/logx"
)

const statusKeyPrefix = "judge:status:"
const (
	defaultStatusCacheTTL      = 30 * time.Minute
	defaultStatusCacheEmptyTTL = 5 * time.Minute
)

// StatusRepository handles status persistence.
type StatusRepository struct {
	cache            cachex.Cache
	submissionsModel dbmodel.SubmissionsModel
	publisher        StatusEventPublisher
	ttl              time.Duration
	emptyTTL         time.Duration
}

// NewStatusRepository creates a new repository.
func NewStatusRepository(cacheClient cachex.Cache, submissionsModel dbmodel.SubmissionsModel, ttl, emptyTTL time.Duration, publisher StatusEventPublisher) *StatusRepository {
	if ttl <= 0 {
		ttl = defaultStatusCacheTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultStatusCacheEmptyTTL
	}
	return &StatusRepository{
		cache:            cacheClient,
		submissionsModel: submissionsModel,
		publisher:        publisher,
		ttl:              ttl,
		emptyTTL:         emptyTTL,
	}
}

// Get returns status by submission id.
func (r *StatusRepository) Get(ctx context.Context, submissionID string) (pmodel.JudgeStatusResponse, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("get status start submission_id=%s", submissionID)
	if submissionID == "" {
		logger.Error("submission_id is required")
		return pmodel.JudgeStatusResponse{}, appErr.ValidationError("submission_id", "required")
	}
	if r.cache == nil {
		return r.getFinalStatusFromDB(ctx, submissionID)
	}

	status, err := cachex.GetWithCached[*pmodel.JudgeStatusResponse](
		ctx,
		r.cache,
		statusKeyPrefix+submissionID,
		cachex.JitterTTL(r.ttl),
		cachex.JitterTTL(r.emptyTTL),
		func(st *pmodel.JudgeStatusResponse) bool { return st == nil },
		marshalStatus,
		unmarshalStatus,
		func(ctx context.Context) (*pmodel.JudgeStatusResponse, error) {
			status, err := r.getFinalStatusFromDB(ctx, submissionID)
			if err != nil {
				if appErr.Is(err, appErr.NotFound) {
					return nil, nil
				}
				return nil, err
			}
			return &status, nil
		},
	)
	if err != nil {
		logger.Errorf("get status failed: %v", err)
		return pmodel.JudgeStatusResponse{}, err
	}
	if status == nil {
		return pmodel.JudgeStatusResponse{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
	}
	return *status, nil
}

// GetBatch returns statuses for multiple submission ids.
func (r *StatusRepository) GetBatch(ctx context.Context, submissionIDs []string) ([]pmodel.JudgeStatusResponse, []string, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("get batch status start total=%d", len(submissionIDs))
	if len(submissionIDs) == 0 {
		logger.Error("submission_ids is required")
		return nil, nil, appErr.ValidationError("submission_ids", "required")
	}
	statuses := make([]pmodel.JudgeStatusResponse, 0, len(submissionIDs))
	missing := make([]string, 0)
	if r.cache != nil {
		keys := make([]string, 0, len(submissionIDs))
		for _, submissionID := range submissionIDs {
			if submissionID == "" {
				logger.Error("submission_id is required")
				return nil, nil, appErr.ValidationError("submission_id", "required")
			}
			keys = append(keys, statusKeyPrefix+submissionID)
		}
		values, err := r.cache.MGet(ctx, keys...)
		if err != nil {
			logger.Errorf("batch get status cache failed: %v", err)
			return nil, nil, appErr.Wrapf(err, appErr.CacheError, "batch get status failed")
		}
		for i, raw := range values {
			if raw == "" {
				missing = append(missing, submissionIDs[i])
				continue
			}
			if raw == cachex.NullCacheValue {
				continue
			}
			var resp pmodel.JudgeStatusResponse
			if err := json.Unmarshal([]byte(raw), &resp); err != nil {
				logger.Errorf("decode status failed: %v", err)
				return nil, nil, appErr.Wrapf(err, appErr.CacheError, "decode status failed")
			}
			statuses = append(statuses, resp)
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
		statuses = append(statuses, dbStatuses...)
		if r.cache != nil {
			for _, st := range dbStatuses {
				if payload := mustMarshalStatus(st); payload != "" {
					_ = r.cache.Set(ctx, statusKeyPrefix+st.SubmissionID, payload, cachex.JitterTTL(r.ttl))
				}
			}
		}
	}
	if r.cache != nil && len(dbMissing) > 0 {
		for _, id := range dbMissing {
			if id == "" {
				continue
			}
			_ = r.cache.Set(ctx, statusKeyPrefix+id, cachex.NullCacheValue, cachex.JitterTTL(r.emptyTTL))
		}
	}
	return statuses, dbMissing, nil
}

// Save persists status.
func (r *StatusRepository) Save(ctx context.Context, status pmodel.JudgeStatusResponse) error {
	logger := logx.WithContext(ctx)
	logger.Infof("save status start submission_id=%s status=%s", status.SubmissionID, status.Status)
	if status.SubmissionID == "" {
		logger.Error("submission_id is required")
		return appErr.ValidationError("submission_id", "required")
	}
	if isFinalStatus(status.Status) {
		if r.publisher == nil {
			logger.Error("status publisher is not configured")
			return appErr.New(appErr.ServiceUnavailable).WithMessage("status publisher is not configured")
		}
		if err := r.publisher.PublishFinalStatus(ctx, status); err != nil {
			logger.Errorf("publish final status failed: %v", err)
			return err
		}
	}
	if r.cache != nil {
		data, err := json.Marshal(status)
		if err != nil {
			logger.Errorf("marshal status failed: %v", err)
			return fmt.Errorf("marshal status failed: %w", err)
		}
		if err := r.cache.Set(ctx, statusKeyPrefix+status.SubmissionID, string(data), cachex.JitterTTL(r.ttl)); err != nil {
			logger.Errorf("store status failed: %v", err)
			return appErr.Wrapf(err, appErr.CacheError, "store status failed")
		}
	}
	return nil
}

// PersistFinalStatus stores final status into the database.
func (r *StatusRepository) PersistFinalStatus(ctx context.Context, status pmodel.JudgeStatusResponse) error {
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

func (r *StatusRepository) storeFinalStatus(ctx context.Context, status pmodel.JudgeStatusResponse) error {
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

func (r *StatusRepository) getFinalStatusFromDB(ctx context.Context, submissionID string) (pmodel.JudgeStatusResponse, error) {
	logger := logx.WithContext(ctx)
	if r.submissionsModel == nil {
		logger.Error("submissions model is not configured")
		return pmodel.JudgeStatusResponse{}, appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
	}
	payload, err := r.submissionsModel.FindFinalStatus(ctx, submissionID)
	if err != nil {
		if errors.Is(err, dbmodel.ErrNotFound) {
			return pmodel.JudgeStatusResponse{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
		}
		return pmodel.JudgeStatusResponse{}, appErr.Wrapf(err, appErr.DatabaseError, "get final status failed")
	}
	var resp pmodel.JudgeStatusResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		logger.Errorf("decode final status failed: %v", err)
		return pmodel.JudgeStatusResponse{}, appErr.Wrapf(err, appErr.DatabaseError, "decode final status failed")
	}
	return resp, nil
}

func (r *StatusRepository) getFinalStatusBatchFromDB(ctx context.Context, submissionIDs []string) ([]pmodel.JudgeStatusResponse, []string, error) {
	logger := logx.WithContext(ctx)
	if r.submissionsModel == nil {
		logger.Error("submissions model is not configured")
		return nil, submissionIDs, appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
	}
	if len(submissionIDs) == 0 {
		return nil, nil, nil
	}
	for _, id := range submissionIDs {
		if id == "" {
			logger.Error("submission_id is required")
			return nil, nil, appErr.ValidationError("submission_id", "required")
		}
	}
	records, err := r.submissionsModel.FindFinalStatusBatch(ctx, submissionIDs)
	if err != nil {
		logger.Errorf("batch get final status failed: %v", err)
		return nil, nil, appErr.Wrapf(err, appErr.DatabaseError, "batch get final status failed")
	}
	found := make(map[string]pmodel.JudgeStatusResponse, len(records))
	for _, record := range records {
		if record.SubmissionID == "" {
			continue
		}
		var resp pmodel.JudgeStatusResponse
		if err := json.Unmarshal([]byte(record.FinalStatus), &resp); err != nil {
			logger.Errorf("decode final status failed: %v", err)
			return nil, nil, appErr.Wrapf(err, appErr.DatabaseError, "decode final status failed")
		}
		resp.SubmissionID = record.SubmissionID
		found[record.SubmissionID] = resp
	}
	statuses := make([]pmodel.JudgeStatusResponse, 0, len(found))
	missing := make([]string, 0)
	for _, id := range submissionIDs {
		if st, ok := found[id]; ok {
			statuses = append(statuses, st)
		} else {
			missing = append(missing, id)
		}
	}
	return statuses, missing, nil
}

func isFinalStatus(status result.JudgeStatus) bool {
	return status == result.StatusFinished || status == result.StatusFailed
}

func mustMarshalStatus(status pmodel.JudgeStatusResponse) string {
	data, err := json.Marshal(status)
	if err != nil {
		return ""
	}
	return string(data)
}

func marshalStatus(status *pmodel.JudgeStatusResponse) string {
	if status == nil {
		return ""
	}
	data, err := json.Marshal(status)
	if err != nil {
		return ""
	}
	return string(data)
}

func unmarshalStatus(data string) (*pmodel.JudgeStatusResponse, error) {
	var resp pmodel.JudgeStatusResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
