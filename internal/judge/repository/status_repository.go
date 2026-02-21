package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	"fuzoj/judge_service/internal/model"
	"fuzoj/judge_service/internal/sandbox/result"
	appErr "fuzoj/pkg/errors"
)

const statusKeyPrefix = "judge:status:"

// StatusRepository handles status persistence.
type StatusRepository struct {
	cache      cache.Cache
	dbProvider db.Provider
	publisher  StatusEventPublisher
	TTL        time.Duration
}

// NewStatusRepository creates a new repository.
func NewStatusRepository(cacheClient cache.Cache, provider db.Provider, ttl time.Duration, publisher StatusEventPublisher) *StatusRepository {
	return &StatusRepository{cache: cacheClient, dbProvider: provider, TTL: ttl, publisher: publisher}
}

// Get returns status by submission id.
func (r *StatusRepository) Get(ctx context.Context, submissionID string) (model.JudgeStatusResponse, error) {
	if submissionID == "" {
		return model.JudgeStatusResponse{}, appErr.ValidationError("submission_id", "required")
	}
	if r.cache != nil {
		val, err := r.cache.Get(ctx, statusKeyPrefix+submissionID)
		if err == nil && val != "" {
			var resp model.JudgeStatusResponse
			if err := json.Unmarshal([]byte(val), &resp); err != nil {
				return model.JudgeStatusResponse{}, appErr.Wrapf(err, appErr.CacheError, "decode status failed")
			}
			return resp, nil
		}
	}
	database, err := db.CurrentDatabase(r.dbProvider)
	if err != nil {
		return model.JudgeStatusResponse{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
	}
	resp, err := r.getFinalStatus(ctx, database, submissionID)
	if err != nil {
		return model.JudgeStatusResponse{}, err
	}
	if r.cache != nil {
		if payload := mustMarshalStatus(resp); payload != "" {
			_ = r.cache.Set(ctx, statusKeyPrefix+submissionID, payload, r.TTL)
		}
	}
	return resp, nil
}

// GetBatch returns statuses for multiple submission ids.
func (r *StatusRepository) GetBatch(ctx context.Context, submissionIDs []string) ([]model.JudgeStatusResponse, []string, error) {
	if len(submissionIDs) == 0 {
		return nil, nil, appErr.ValidationError("submission_ids", "required")
	}
	statuses := make([]model.JudgeStatusResponse, 0, len(submissionIDs))
	missing := make([]string, 0)
	if r.cache != nil {
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
	} else {
		missing = append(missing, submissionIDs...)
	}
	database, err := db.CurrentDatabase(r.dbProvider)
	if err == nil && len(missing) > 0 {
		dbStatuses, dbMissing, err := r.getFinalStatusBatch(ctx, database, missing)
		if err != nil {
			return nil, nil, err
		}
		if len(dbStatuses) > 0 {
			statuses = append(statuses, dbStatuses...)
			if r.cache != nil {
				for _, st := range dbStatuses {
					if payload := mustMarshalStatus(st); payload != "" {
						_ = r.cache.Set(ctx, statusKeyPrefix+st.SubmissionID, payload, r.TTL)
					}
				}
			}
		}
		missing = dbMissing
	}
	return statuses, missing, nil
}

// Save persists status.
func (r *StatusRepository) Save(ctx context.Context, status model.JudgeStatusResponse) error {
	if status.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if isFinalStatus(status.Status) {
		if r.publisher == nil {
			return appErr.New(appErr.ServiceUnavailable).WithMessage("status publisher is not configured")
		}
		if err := r.publisher.PublishFinalStatus(ctx, status); err != nil {
			return err
		}
	}
	if r.cache != nil {
		data, err := json.Marshal(status)
		if err != nil {
			return fmt.Errorf("marshal status failed: %w", err)
		}
		if err := r.cache.Set(ctx, statusKeyPrefix+status.SubmissionID, string(data), r.TTL); err != nil {
			return appErr.Wrapf(err, appErr.CacheError, "store status failed")
		}
	}
	return nil
}

// PersistFinalStatus stores final status into the database.
func (r *StatusRepository) PersistFinalStatus(ctx context.Context, status model.JudgeStatusResponse) error {
	if status.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if !isFinalStatus(status.Status) {
		return appErr.ValidationError("status", "final_required")
	}
	database, err := db.CurrentDatabase(r.dbProvider)
	if err != nil {
		return err
	}
	return r.storeFinalStatus(ctx, database, status)
}

func (r *StatusRepository) storeFinalStatus(ctx context.Context, database db.Database, status model.JudgeStatusResponse) error {
	if database == nil {
		return appErr.New(appErr.NotFound).WithMessage("submission status not found")
	}
	payload, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal final status failed: %w", err)
	}
	finishedAt := time.Now()
	if status.Timestamps.FinishedAt > 0 {
		finishedAt = time.Unix(status.Timestamps.FinishedAt, 0)
	}
	query := `
		UPDATE submissions
		SET final_status = ?, final_status_at = ?
		WHERE submission_id = ?
	`
	res, err := database.Exec(ctx, query, string(payload), finishedAt, status.SubmissionID)
	if err != nil {
		return appErr.Wrapf(err, appErr.DatabaseError, "store final status failed")
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
	}
	return nil
}

func (r *StatusRepository) getFinalStatus(ctx context.Context, database db.Database, submissionID string) (model.JudgeStatusResponse, error) {
	if database == nil {
		return model.JudgeStatusResponse{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
	}
	query := `
		SELECT final_status
		FROM submissions
		WHERE submission_id = ? AND final_status IS NOT NULL
		LIMIT 1
	`
	row := database.QueryRow(ctx, query, submissionID)
	var payload string
	if err := row.Scan(&payload); err != nil {
		if db.IsNoRows(err) {
			return model.JudgeStatusResponse{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
		}
		return model.JudgeStatusResponse{}, appErr.Wrapf(err, appErr.DatabaseError, "get final status failed")
	}
	var resp model.JudgeStatusResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		return model.JudgeStatusResponse{}, appErr.Wrapf(err, appErr.DatabaseError, "decode final status failed")
	}
	return resp, nil
}

func (r *StatusRepository) getFinalStatusBatch(ctx context.Context, database db.Database, submissionIDs []string) ([]model.JudgeStatusResponse, []string, error) {
	if database == nil {
		return nil, submissionIDs, nil
	}
	if len(submissionIDs) == 0 {
		return nil, nil, nil
	}
	placeholders := make([]string, 0, len(submissionIDs))
	args := make([]interface{}, 0, len(submissionIDs))
	for _, id := range submissionIDs {
		if id == "" {
			return nil, nil, appErr.ValidationError("submission_id", "required")
		}
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf(
		"SELECT submission_id, final_status FROM submissions WHERE submission_id IN (%s) AND final_status IS NOT NULL",
		strings.Join(placeholders, ","),
	)
	rows, err := database.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, appErr.Wrapf(err, appErr.DatabaseError, "batch get final status failed")
	}
	defer rows.Close()

	found := make(map[string]model.JudgeStatusResponse, len(submissionIDs))
	for rows.Next() {
		var submissionID string
		var payload string
		if err := rows.Scan(&submissionID, &payload); err != nil {
			return nil, nil, appErr.Wrapf(err, appErr.DatabaseError, "scan final status failed")
		}
		var resp model.JudgeStatusResponse
		if err := json.Unmarshal([]byte(payload), &resp); err != nil {
			return nil, nil, appErr.Wrapf(err, appErr.DatabaseError, "decode final status failed")
		}
		if submissionID != "" {
			resp.SubmissionID = submissionID
			found[submissionID] = resp
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, appErr.Wrapf(err, appErr.DatabaseError, "iterate final status failed")
	}

	statuses := make([]model.JudgeStatusResponse, 0, len(found))
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

func mustMarshalStatus(status model.JudgeStatusResponse) string {
	data, err := json.Marshal(status)
	if err != nil {
		return ""
	}
	return string(data)
}
