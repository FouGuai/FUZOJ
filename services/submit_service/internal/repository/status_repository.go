package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statusrepo"
	"fuzoj/pkg/submit/statusutil"
	"fuzoj/pkg/submit/statuswriter"
	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// StatusRepository is shared implementation configured for submit service.
type StatusRepository = statusrepo.StatusRepository[domain.JudgeStatusPayload]

// NewStatusRepository creates a new repository.
func NewStatusRepository(redisClient *redis.Redis, submissionsModel model.SubmissionsModel, ttl, emptyTTL time.Duration) *StatusRepository {
	return statusrepo.NewStatusRepository(statusrepo.StatusRepositoryConfig[domain.JudgeStatusPayload]{
		Cache:    redisClient,
		TTL:      ttl,
		EmptyTTL: emptyTTL,
		GetSubmissionID: func(status domain.JudgeStatusPayload) string {
			return status.SubmissionID
		},
		GetStatusLabel: func(status domain.JudgeStatusPayload) string {
			return status.Status
		},
		Encode: func(status domain.JudgeStatusPayload) (string, error) {
			payload, err := json.Marshal(statusSummary(status))
			if err != nil {
				return "", err
			}
			return string(payload), nil
		},
		Decode: func(raw string) (domain.JudgeStatusPayload, error) {
			var status domain.JudgeStatusPayload
			if err := json.Unmarshal([]byte(raw), &status); err != nil {
				return domain.JudgeStatusPayload{}, err
			}
			return status, nil
		},
		BuildUnknown: unknownStatus,
		LoadOneFromDB: func(ctx context.Context, submissionID string) (domain.JudgeStatusPayload, bool, error) {
			if submissionsModel == nil {
				return domain.JudgeStatusPayload{}, false, appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
			}
			payload, err := submissionsModel.FindFinalStatus(ctx, submissionID)
			if err != nil {
				if errors.Is(err, model.ErrNotFound) {
					return domain.JudgeStatusPayload{}, false, nil
				}
				return domain.JudgeStatusPayload{}, false, appErr.Wrapf(err, appErr.DatabaseError, "get submission status failed")
			}
			var status domain.JudgeStatusPayload
			if err := json.Unmarshal([]byte(payload), &status); err != nil {
				return domain.JudgeStatusPayload{}, false, appErr.Wrapf(err, appErr.DatabaseError, "decode status failed")
			}
			status.SubmissionID = submissionID
			return statusSummary(status), true, nil
		},
		LoadBatchFromDB: func(ctx context.Context, submissionIDs []string) (map[string]domain.JudgeStatusPayload, error) {
			if submissionsModel == nil {
				return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
			}
			rows, err := submissionsModel.FindFinalStatusBatch(ctx, submissionIDs)
			if err != nil {
				return nil, appErr.Wrapf(err, appErr.DatabaseError, "batch get submission status failed")
			}
			resultMap := make(map[string]domain.JudgeStatusPayload, len(rows))
			for _, row := range rows {
				if row.SubmissionID == "" {
					continue
				}
				var status domain.JudgeStatusPayload
				if err := json.Unmarshal([]byte(row.FinalStatus), &status); err != nil {
					return nil, appErr.Wrapf(err, appErr.DatabaseError, "decode status failed")
				}
				status.SubmissionID = row.SubmissionID
				resultMap[row.SubmissionID] = statusSummary(status)
			}
			return resultMap, nil
		},
		LoadFinalFromDB: func(ctx context.Context, submissionID string) (domain.JudgeStatusPayload, bool, error) {
			if submissionsModel == nil {
				return domain.JudgeStatusPayload{}, false, appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
			}
			payload, err := submissionsModel.FindFinalStatus(ctx, submissionID)
			if err != nil {
				if errors.Is(err, model.ErrNotFound) {
					return domain.JudgeStatusPayload{}, false, nil
				}
				return domain.JudgeStatusPayload{}, false, appErr.Wrapf(err, appErr.DatabaseError, "get submission status failed")
			}
			var status domain.JudgeStatusPayload
			if err := json.Unmarshal([]byte(payload), &status); err != nil {
				return domain.JudgeStatusPayload{}, false, appErr.Wrapf(err, appErr.DatabaseError, "decode status failed")
			}
			status.SubmissionID = submissionID
			return status, true, nil
		},
		ToWriterPayload: func(status domain.JudgeStatusPayload) (statuswriter.StatusPayload, error) {
			return toStatusWriterPayload(status), nil
		},
		IsFinalStatus: func(status domain.JudgeStatusPayload) bool {
			return statusutil.IsFinalStatus(status.Status)
		},
		PersistFinal: func(ctx context.Context, status domain.JudgeStatusPayload) error {
			if submissionsModel == nil {
				return appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
			}
			if status.SubmissionID == "" {
				return appErr.ValidationError("submission_id", "required")
			}
			if !statusutil.IsFinalStatus(status.Status) {
				return appErr.ValidationError("status", "final_required")
			}
			payload, err := json.Marshal(status)
			if err != nil {
				return fmt.Errorf("marshal final status failed: %w", err)
			}
			finishedAt := time.Now()
			if status.Timestamps.FinishedAt > 0 {
				finishedAt = time.Unix(status.Timestamps.FinishedAt, 0)
			}
			res, err := submissionsModel.UpdateFinalStatus(ctx, status.SubmissionID, string(payload), finishedAt)
			if err != nil {
				return appErr.Wrapf(err, appErr.DatabaseError, "store final status failed")
			}
			affected, err := res.RowsAffected()
			if err == nil && affected == 0 {
				if _, findErr := submissionsModel.FindOne(ctx, status.SubmissionID); findErr != nil {
					if errors.Is(findErr, model.ErrNotFound) {
						return appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
					}
					return appErr.Wrapf(findErr, appErr.DatabaseError, "check submission existence failed")
				}
				return nil
			}
			return nil
		},
	})
}

func statusSummary(status domain.JudgeStatusPayload) domain.JudgeStatusPayload {
	summary := statuswriter.BuildSummary(toStatusWriterPayload(status))
	return fromStatusWriterPayload(summary)
}

func toStatusWriterPayload(status domain.JudgeStatusPayload) statuswriter.StatusPayload {
	return statuswriter.StatusPayload{
		SubmissionID: status.SubmissionID,
		Status:       status.Status,
		Verdict:      status.Verdict,
		Score:        status.Score,
		Language:     status.Language,
		Summary: statuswriter.SummaryStat{
			TotalTimeMs:  status.Summary.TotalTimeMs,
			MaxMemoryKB:  status.Summary.MaxMemoryKB,
			TotalScore:   status.Summary.TotalScore,
			FailedTestID: status.Summary.FailedTestID,
		},
		Compile: (*statuswriter.CompileResult)(status.Compile),
		Tests:   castTests(status.Tests),
		Timestamps: statuswriter.Timestamps{
			ReceivedAt: status.Timestamps.ReceivedAt,
			FinishedAt: status.Timestamps.FinishedAt,
		},
		Progress: statuswriter.Progress{
			TotalTests: status.Progress.TotalTests,
			DoneTests:  status.Progress.DoneTests,
		},
		ErrorCode:    status.ErrorCode,
		ErrorMessage: status.ErrorMessage,
	}
}

func fromStatusWriterPayload(status statuswriter.StatusPayload) domain.JudgeStatusPayload {
	return domain.JudgeStatusPayload{
		SubmissionID: status.SubmissionID,
		Status:       status.Status,
		Verdict:      status.Verdict,
		Score:        status.Score,
		Language:     status.Language,
		Summary: domain.SummaryStat{
			TotalTimeMs:  status.Summary.TotalTimeMs,
			MaxMemoryKB:  status.Summary.MaxMemoryKB,
			TotalScore:   status.Summary.TotalScore,
			FailedTestID: status.Summary.FailedTestID,
		},
		Compile: (*domain.CompileResult)(status.Compile),
		Tests:   castDomainTests(status.Tests),
		Timestamps: domain.Timestamps{
			ReceivedAt: status.Timestamps.ReceivedAt,
			FinishedAt: status.Timestamps.FinishedAt,
		},
		Progress: domain.Progress{
			TotalTests: status.Progress.TotalTests,
			DoneTests:  status.Progress.DoneTests,
		},
		ErrorCode:    status.ErrorCode,
		ErrorMessage: status.ErrorMessage,
	}
}

func castTests(in []domain.TestcaseResult) []statuswriter.TestcaseResult {
	if len(in) == 0 {
		return nil
	}
	out := make([]statuswriter.TestcaseResult, 0, len(in))
	for _, t := range in {
		out = append(out, statuswriter.TestcaseResult{
			TestID:     t.TestID,
			Verdict:    t.Verdict,
			TimeMs:     t.TimeMs,
			MemoryKB:   t.MemoryKB,
			OutputKB:   t.OutputKB,
			ExitCode:   t.ExitCode,
			RuntimeLog: t.RuntimeLog,
			CheckerLog: t.CheckerLog,
			Stdout:     t.Stdout,
			Stderr:     t.Stderr,
			Score:      t.Score,
			SubtaskID:  t.SubtaskID,
		})
	}
	return out
}

func castDomainTests(in []statuswriter.TestcaseResult) []domain.TestcaseResult {
	if len(in) == 0 {
		return nil
	}
	out := make([]domain.TestcaseResult, 0, len(in))
	for _, t := range in {
		out = append(out, domain.TestcaseResult{
			TestID:     t.TestID,
			Verdict:    t.Verdict,
			TimeMs:     t.TimeMs,
			MemoryKB:   t.MemoryKB,
			OutputKB:   t.OutputKB,
			ExitCode:   t.ExitCode,
			RuntimeLog: t.RuntimeLog,
			CheckerLog: t.CheckerLog,
			Stdout:     t.Stdout,
			Stderr:     t.Stderr,
			Score:      t.Score,
			SubtaskID:  t.SubtaskID,
		})
	}
	return out
}

func ttlSeconds(ttl time.Duration) int {
	return statusutil.TTLSeconds(ttl)
}

func unknownStatus(submissionID string) domain.JudgeStatusPayload {
	return domain.JudgeStatusPayload{
		SubmissionID: submissionID,
		Status:       "Unknown",
	}
}
