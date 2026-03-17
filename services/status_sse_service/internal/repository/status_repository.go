package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statuscache"
	"fuzoj/pkg/submit/statuswriter"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	ownerCachePrefix      = "status:sse:owner:"
	ownerCacheMissValue   = "$NULL$"
	defaultOwnerCacheTTL  = 5 * time.Minute
	defaultOwnerMissTTL   = time.Minute
	statusFinishedLiteral = "finished"
	statusFailedLiteral   = "failed"
)

type StatusRepository struct {
	conn         sqlx.SqlConn
	cache        *redis.Redis
	ownerTTL     time.Duration
	ownerMissTTL time.Duration
}

func NewStatusRepository(conn sqlx.SqlConn, cacheClient *redis.Redis, ownerTTL, ownerMissTTL time.Duration) *StatusRepository {
	if ownerTTL <= 0 {
		ownerTTL = defaultOwnerCacheTTL
	}
	if ownerMissTTL <= 0 {
		ownerMissTTL = defaultOwnerMissTTL
	}
	return &StatusRepository{
		conn:         conn,
		cache:        cacheClient,
		ownerTTL:     ownerTTL,
		ownerMissTTL: ownerMissTTL,
	}
}

func (r *StatusRepository) CheckSubmissionOwner(ctx context.Context, submissionID string, userID int64) error {
	if strings.TrimSpace(submissionID) == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if userID <= 0 {
		return appErr.New(appErr.Unauthorized).WithMessage("user is not authenticated")
	}
	ownerID, err := r.getSubmissionOwnerID(ctx, submissionID)
	if err != nil {
		return err
	}
	if ownerID != userID {
		return appErr.New(appErr.PermissionDenied).WithMessage("submission access denied")
	}
	return nil
}

func (r *StatusRepository) GetLatestStatus(ctx context.Context, submissionID string) (statuswriter.StatusPayload, error) {
	if strings.TrimSpace(submissionID) == "" {
		return statuswriter.StatusPayload{}, appErr.ValidationError("submission_id", "required")
	}
	if r.cache != nil {
		cached, hit, err := statuscache.Get(ctx, r.cache, submissionID)
		if err != nil {
			return statuswriter.StatusPayload{}, appErr.Wrapf(err, appErr.CacheError, "get status cache failed")
		}
		if hit {
			if cached == statuscache.NullValue {
				return statuswriter.StatusPayload{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
			}
			status, err := unmarshalStatus(cached)
			if err == nil {
				return status, nil
			}
		}
	}
	return r.getFinalStatusFromDB(ctx, submissionID)
}

func (r *StatusRepository) GetFinalStatus(ctx context.Context, submissionID string) (statuswriter.StatusPayload, error) {
	if strings.TrimSpace(submissionID) == "" {
		return statuswriter.StatusPayload{}, appErr.ValidationError("submission_id", "required")
	}
	return r.getFinalStatusFromDB(ctx, submissionID)
}

func (r *StatusRepository) getSubmissionOwnerID(ctx context.Context, submissionID string) (int64, error) {
	if r.cache != nil {
		key := ownerCachePrefix + submissionID
		value, err := r.cache.GetCtx(ctx, key)
		if err == nil && value != "" {
			if value == ownerCacheMissValue {
				return 0, appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
			}
			ownerID, convErr := parseOwnerID(value)
			if convErr == nil {
				return ownerID, nil
			}
		}
	}

	query := "select user_id from submissions where submission_id = ? limit 1"
	var ownerID int64
	err := r.conn.QueryRowCtx(ctx, &ownerID, query, submissionID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			r.cacheOwner(submissionID, ownerCacheMissValue, r.ownerMissTTL)
			return 0, appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
		}
		return 0, appErr.Wrapf(err, appErr.DatabaseError, "get submission owner failed")
	}
	r.cacheOwner(submissionID, fmt.Sprintf("%d", ownerID), r.ownerTTL)
	return ownerID, nil
}

func (r *StatusRepository) getFinalStatusFromDB(ctx context.Context, submissionID string) (statuswriter.StatusPayload, error) {
	query := "select final_status from submissions where submission_id = ? and final_status is not null limit 1"
	var payload string
	err := r.conn.QueryRowCtx(ctx, &payload, query, submissionID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			if r.cache != nil {
				_ = statuscache.Set(ctx, r.cache, submissionID, statuscache.NullValue, ttlSeconds(defaultOwnerMissTTL))
			}
			return statuswriter.StatusPayload{}, appErr.New(appErr.NotFound).WithMessage("submission status not found")
		}
		return statuswriter.StatusPayload{}, appErr.Wrapf(err, appErr.DatabaseError, "get submission status failed")
	}
	status, err := unmarshalStatus(payload)
	if err != nil {
		return statuswriter.StatusPayload{}, appErr.Wrapf(err, appErr.DatabaseError, "decode status failed")
	}
	return status, nil
}

func (r *StatusRepository) cacheOwner(submissionID, value string, ttl time.Duration) {
	if r == nil || r.cache == nil || strings.TrimSpace(submissionID) == "" {
		return
	}
	seconds := ttlSeconds(ttl)
	if seconds <= 0 {
		return
	}
	_ = r.cache.SetexCtx(context.Background(), ownerCachePrefix+submissionID, value, seconds)
}

func parseOwnerID(value string) (int64, error) {
	var ownerID int64
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &ownerID)
	if err != nil || ownerID <= 0 {
		return 0, fmt.Errorf("invalid owner id")
	}
	return ownerID, nil
}

func unmarshalStatus(raw string) (statuswriter.StatusPayload, error) {
	var status statuswriter.StatusPayload
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		return statuswriter.StatusPayload{}, err
	}
	return status, nil
}

func IsFinalStatus(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == statusFinishedLiteral || s == statusFailedLiteral
}

func BuildSummary(status statuswriter.StatusPayload) statuswriter.StatusPayload {
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
