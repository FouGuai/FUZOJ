package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statusrepo"
	"fuzoj/pkg/submit/statusutil"
	"fuzoj/pkg/submit/statuswriter"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	ownerCachePrefix    = "status:sse:owner:"
	ownerCacheMissValue = "$NULL$"
)

// StatusRepository is shared implementation configured for status SSE service.
type StatusRepository = statusrepo.StatusRepository[statuswriter.StatusPayload]

func NewStatusRepository(conn sqlx.SqlConn, cacheClient *redis.Redis, ownerTTL, ownerMissTTL time.Duration) *StatusRepository {
	if ownerTTL <= 0 {
		ownerTTL = 5 * time.Minute
	}
	if ownerMissTTL <= 0 {
		ownerMissTTL = time.Minute
	}
	return statusrepo.NewStatusRepository(statusrepo.StatusRepositoryConfig[statuswriter.StatusPayload]{
		Cache:    cacheClient,
		TTL:      ownerTTL,
		EmptyTTL: ownerMissTTL,
		GetSubmissionID: func(status statuswriter.StatusPayload) string {
			return status.SubmissionID
		},
		GetStatusLabel: func(status statuswriter.StatusPayload) string {
			return status.Status
		},
		Encode: func(status statuswriter.StatusPayload) (string, error) {
			data, err := json.Marshal(status)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		Decode: func(raw string) (statuswriter.StatusPayload, error) {
			var status statuswriter.StatusPayload
			if err := json.Unmarshal([]byte(raw), &status); err != nil {
				return statuswriter.StatusPayload{}, err
			}
			return status, nil
		},
		BuildUnknown: func(submissionID string) statuswriter.StatusPayload {
			return statuswriter.StatusPayload{SubmissionID: submissionID, Status: "Unknown"}
		},
		LoadOneFromDB: func(ctx context.Context, submissionID string) (statuswriter.StatusPayload, bool, error) {
			query := "select final_status from submissions where submission_id = ? and final_status is not null limit 1"
			var payload string
			err := conn.QueryRowCtx(ctx, &payload, query, submissionID)
			if err != nil {
				if errors.Is(err, sqlx.ErrNotFound) {
					return statuswriter.StatusPayload{}, false, nil
				}
				return statuswriter.StatusPayload{}, false, appErr.Wrapf(err, appErr.DatabaseError, "get submission status failed")
			}
			var status statuswriter.StatusPayload
			if err := json.Unmarshal([]byte(payload), &status); err != nil {
				return statuswriter.StatusPayload{}, false, appErr.Wrapf(err, appErr.DatabaseError, "decode status failed")
			}
			status.SubmissionID = submissionID
			return status, true, nil
		},
		LoadBatchFromDB: func(ctx context.Context, submissionIDs []string) (map[string]statuswriter.StatusPayload, error) {
			resultMap := make(map[string]statuswriter.StatusPayload, len(submissionIDs))
			for _, id := range submissionIDs {
				if id == "" {
					continue
				}
				status, found, err := queryFinalStatus(ctx, conn, id)
				if err != nil {
					return nil, err
				}
				if found {
					resultMap[id] = status
				}
			}
			return resultMap, nil
		},
		LoadFinalFromDB: func(ctx context.Context, submissionID string) (statuswriter.StatusPayload, bool, error) {
			return queryFinalStatus(ctx, conn, submissionID)
		},
		CheckOwner: func(ctx context.Context, submissionID string, userID int64) error {
			if strings.TrimSpace(submissionID) == "" {
				return appErr.ValidationError("submission_id", "required")
			}
			if userID <= 0 {
				return appErr.New(appErr.Unauthorized).WithMessage("user is not authenticated")
			}
			ownerID, err := getSubmissionOwnerID(ctx, conn, cacheClient, submissionID, ownerTTL, ownerMissTTL)
			if err != nil {
				return err
			}
			if ownerID != userID {
				return appErr.New(appErr.PermissionDenied).WithMessage("submission access denied")
			}
			return nil
		},
	})
}

func queryFinalStatus(ctx context.Context, conn sqlx.SqlConn, submissionID string) (statuswriter.StatusPayload, bool, error) {
	query := "select final_status from submissions where submission_id = ? and final_status is not null limit 1"
	var payload string
	err := conn.QueryRowCtx(ctx, &payload, query, submissionID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return statuswriter.StatusPayload{}, false, nil
		}
		return statuswriter.StatusPayload{}, false, appErr.Wrapf(err, appErr.DatabaseError, "get submission status failed")
	}
	var status statuswriter.StatusPayload
	if err := json.Unmarshal([]byte(payload), &status); err != nil {
		return statuswriter.StatusPayload{}, false, appErr.Wrapf(err, appErr.DatabaseError, "decode status failed")
	}
	status.SubmissionID = submissionID
	return status, true, nil
}

func getSubmissionOwnerID(ctx context.Context, conn sqlx.SqlConn, cacheClient *redis.Redis, submissionID string, ownerTTL, ownerMissTTL time.Duration) (int64, error) {
	if cacheClient != nil {
		key := ownerCachePrefix + submissionID
		value, err := cacheClient.GetCtx(ctx, key)
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
	err := conn.QueryRowCtx(ctx, &ownerID, query, submissionID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			cacheOwner(cacheClient, submissionID, ownerCacheMissValue, ownerMissTTL)
			return 0, appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
		}
		return 0, appErr.Wrapf(err, appErr.DatabaseError, "get submission owner failed")
	}
	cacheOwner(cacheClient, submissionID, fmt.Sprintf("%d", ownerID), ownerTTL)
	return ownerID, nil
}

func cacheOwner(cacheClient *redis.Redis, submissionID, value string, ttl time.Duration) {
	if cacheClient == nil || strings.TrimSpace(submissionID) == "" {
		return
	}
	seconds := statusutil.TTLSeconds(ttl)
	if seconds <= 0 {
		return
	}
	_ = cacheClient.SetexCtx(context.Background(), ownerCachePrefix+submissionID, value, seconds)
}

func parseOwnerID(value string) (int64, error) {
	var ownerID int64
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &ownerID)
	if err != nil || ownerID <= 0 {
		return 0, fmt.Errorf("invalid owner id")
	}
	return ownerID, nil
}

func IsFinalStatus(status string) bool {
	return statusutil.IsFinalStatus(status)
}

func BuildSummary(status statuswriter.StatusPayload) statuswriter.StatusPayload {
	return statuswriter.BuildSummary(status)
}
