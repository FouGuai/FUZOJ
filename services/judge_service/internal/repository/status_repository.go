package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statusrepo"
	"fuzoj/pkg/submit/statusutil"
	"fuzoj/pkg/submit/statuswriter"
	dbmodel "fuzoj/services/judge_service/internal/model"
	"fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/sandbox/result"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// StatusRepository is shared implementation configured for judge service.
type StatusRepository = statusrepo.StatusRepository[pmodel.JudgeStatusResponse]

// FinalStatusEnqueuer abstracts final status batching.
type FinalStatusEnqueuer interface {
	Enqueue(ctx context.Context, status pmodel.JudgeStatusResponse) error
}

// NewStatusRepository creates a new repository.
func NewStatusRepository(cacheClient *redis.Redis, submissionsModel dbmodel.SubmissionsModel, ttl, emptyTTL time.Duration, batcher FinalStatusEnqueuer) *StatusRepository {
	return statusrepo.NewStatusRepository(statusrepo.StatusRepositoryConfig[pmodel.JudgeStatusResponse]{
		Cache:    cacheClient,
		TTL:      ttl,
		EmptyTTL: emptyTTL,
		GetSubmissionID: func(status pmodel.JudgeStatusResponse) string {
			return status.SubmissionID
		},
		GetStatusLabel: func(status pmodel.JudgeStatusResponse) string {
			return string(status.Status)
		},
		Encode: func(status pmodel.JudgeStatusResponse) (string, error) {
			data, err := json.Marshal(status)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		Decode: func(raw string) (pmodel.JudgeStatusResponse, error) {
			var status pmodel.JudgeStatusResponse
			if err := json.Unmarshal([]byte(raw), &status); err != nil {
				return pmodel.JudgeStatusResponse{}, err
			}
			return status, nil
		},
		BuildUnknown: func(submissionID string) pmodel.JudgeStatusResponse {
			return pmodel.JudgeStatusResponse{SubmissionID: submissionID, Status: result.JudgeStatus("Unknown")}
		},
		LoadOneFromDB: func(ctx context.Context, submissionID string) (pmodel.JudgeStatusResponse, bool, error) {
			if submissionsModel == nil {
				return pmodel.JudgeStatusResponse{}, false, appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
			}
			payload, err := submissionsModel.FindFinalStatus(ctx, submissionID)
			if err != nil {
				if err == dbmodel.ErrNotFound {
					return pmodel.JudgeStatusResponse{}, false, nil
				}
				return pmodel.JudgeStatusResponse{}, false, appErr.Wrapf(err, appErr.DatabaseError, "get final status failed")
			}
			var status pmodel.JudgeStatusResponse
			if err := json.Unmarshal([]byte(payload), &status); err != nil {
				return pmodel.JudgeStatusResponse{}, false, appErr.Wrapf(err, appErr.DatabaseError, "decode final status failed")
			}
			status.SubmissionID = submissionID
			return status, true, nil
		},
		LoadBatchFromDB: func(ctx context.Context, submissionIDs []string) (map[string]pmodel.JudgeStatusResponse, error) {
			if submissionsModel == nil {
				return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
			}
			records, err := submissionsModel.FindFinalStatusBatch(ctx, submissionIDs)
			if err != nil {
				return nil, appErr.Wrapf(err, appErr.DatabaseError, "batch get final status failed")
			}
			resultMap := make(map[string]pmodel.JudgeStatusResponse, len(records))
			for _, record := range records {
				if record.SubmissionID == "" {
					continue
				}
				var status pmodel.JudgeStatusResponse
				if err := json.Unmarshal([]byte(record.FinalStatus), &status); err != nil {
					return nil, appErr.Wrapf(err, appErr.DatabaseError, "decode final status failed")
				}
				status.SubmissionID = record.SubmissionID
				resultMap[record.SubmissionID] = status
			}
			return resultMap, nil
		},
		LoadFinalFromDB: nil,
		ToWriterPayload: func(status pmodel.JudgeStatusResponse) (statuswriter.StatusPayload, error) {
			return toStatusWriterPayload(status), nil
		},
		IsFinalStatus: func(status pmodel.JudgeStatusResponse) bool {
			return statusutil.IsFinalStatus(string(status.Status))
		},
		OnFinalStatus: func(ctx context.Context, status pmodel.JudgeStatusResponse) error {
			if batcher == nil {
				return nil
			}
			return batcher.Enqueue(ctx, status)
		},
		PersistFinal: func(ctx context.Context, status pmodel.JudgeStatusResponse) error {
			if submissionsModel == nil {
				return appErr.New(appErr.ServiceUnavailable).WithMessage("submissions model is not configured")
			}
			if status.SubmissionID == "" {
				return appErr.ValidationError("submission_id", "required")
			}
			if !statusutil.IsFinalStatus(string(status.Status)) {
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
				return nil
			}
			return nil
		},
	})
}

func toStatusWriterPayload(status pmodel.JudgeStatusResponse) statuswriter.StatusPayload {
	return statuswriter.StatusPayload{
		SubmissionID: status.SubmissionID,
		Status:       string(status.Status),
		Verdict:      string(status.Verdict),
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

func castTests(in []result.TestcaseResult) []statuswriter.TestcaseResult {
	if len(in) == 0 {
		return nil
	}
	out := make([]statuswriter.TestcaseResult, 0, len(in))
	for _, t := range in {
		out = append(out, statuswriter.TestcaseResult{
			TestID:     t.TestID,
			Verdict:    string(t.Verdict),
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

func isFinalStatus(status result.JudgeStatus) bool {
	return statusutil.IsFinalStatus(string(status))
}
