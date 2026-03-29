package statuswriter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statuscache"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	defaultStatusTTL    = 24 * time.Hour
	statusFinishedValue = "Finished"
	statusFailedValue   = "Failed"
)

// FinalStatusWriter stores final status into database and redis cache.
type FinalStatusWriter struct {
	conn  sqlx.SqlConn
	redis *redis.Redis
	ttl   time.Duration
}

// NewFinalStatusWriter creates a writer for final status persistence.
func NewFinalStatusWriter(conn sqlx.SqlConn, redisClient *redis.Redis, ttl time.Duration) *FinalStatusWriter {
	if ttl <= 0 {
		ttl = defaultStatusTTL
	}
	return &FinalStatusWriter{
		conn:  conn,
		redis: redisClient,
		ttl:   ttl,
	}
}

// WriteFinalStatus writes final status payload to database and cache.
func (w *FinalStatusWriter) WriteFinalStatus(ctx context.Context, status StatusPayload) error {
	logger := logx.WithContext(ctx)
	logger.Infof("write final status start submission_id=%s", status.SubmissionID)
	if status.SubmissionID == "" {
		logger.Error("submission_id is required")
		return appErr.ValidationError("submission_id", "required")
	}
	if !isFinalStatus(status.Status) {
		logger.Error("status must be final")
		return appErr.ValidationError("status", "final_required")
	}

	payload, err := json.Marshal(status)
	if err != nil {
		logger.Errorf("marshal final status failed: %v", err)
		return fmt.Errorf("marshal final status failed: %w", err)
	}
	finishedAt := time.Now()
	if status.Timestamps.FinishedAt > 0 {
		finishedAt = time.Unix(status.Timestamps.FinishedAt, 0)
	}
	res, err := w.conn.ExecCtx(ctx, "update `submissions` set `final_status` = ?, `final_status_at` = ? where `submission_id` = ? and `final_status_at` is null", string(payload), finishedAt, status.SubmissionID)
	if err != nil {
		logger.Errorf("store final status failed: %v", err)
		return appErr.Wrapf(err, appErr.DatabaseError, "store final status failed")
	}
	if res != nil {
		affected, err := res.RowsAffected()
		if err == nil && affected == 0 {
			exists, existsErr := w.submissionExists(ctx, status.SubmissionID)
			if existsErr != nil {
				return existsErr
			}
			if !exists {
				logger.Error("submission not found")
				return appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
			}
			logger.Infof("skip duplicate final status submission_id=%s", status.SubmissionID)
			return nil
		}
	}

	if w.redis != nil {
		summary := status
		summary.Compile = nil
		summary.Tests = nil
		if data, err := json.Marshal(summary); err == nil {
			if err := statuscache.Set(ctx, w.redis, status.SubmissionID, string(data), ttlSeconds(w.ttl)); err != nil {
				logger.Errorf("store status cache failed: %v", err)
				return appErr.Wrapf(err, appErr.CacheError, "store status cache failed")
			}
		}
	}
	return nil
}

func (w *FinalStatusWriter) submissionExists(ctx context.Context, submissionID string) (bool, error) {
	var marker int
	if err := w.conn.QueryRowCtx(ctx, &marker, "select 1 from `submissions` where `submission_id` = ? limit 1", submissionID); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return false, nil
		}
		return false, appErr.Wrapf(err, appErr.DatabaseError, "check submission existence failed")
	}
	return true, nil
}

func isFinalStatus(status string) bool {
	return status == statusFinishedValue || status == statusFailedValue
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
