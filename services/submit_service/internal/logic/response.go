package logic

import (
	"context"
	"fmt"
	"time"

	"fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/repository"
	"fuzoj/services/submit_service/internal/types"
)

func buildCreateResponse(ctx context.Context, submissionID string, status domain.JudgeStatusPayload) *types.CreateSubmissionResponse {
	return &types.CreateSubmissionResponse{
		Code:    int(errors.Success),
		Message: "Success",
		Data: types.CreateSubmissionData{
			SubmissionId: submissionID,
			Status:       status.Status,
			ReceivedAt:   status.Timestamps.ReceivedAt,
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildStatusResponse(ctx context.Context, status domain.JudgeStatusPayload) *types.GetStatusResponse {
	return &types.GetStatusResponse{
		Code:    int(errors.Success),
		Message: "Success",
		Data:    toJudgeStatusData(status),
		TraceId: traceIDFromContext(ctx),
	}
}

func buildBatchStatusResponse(ctx context.Context, statuses []domain.JudgeStatusPayload, missing []string) *types.BatchStatusResponse {
	items := make([]types.JudgeStatusData, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, toJudgeStatusData(status))
	}
	return &types.BatchStatusResponse{
		Code:    int(errors.Success),
		Message: "Success",
		Data: types.BatchStatusData{
			Items:   items,
			Missing: missing,
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildSourceResponse(ctx context.Context, submission *repository.Submission) *types.GetSourceResponse {
	data := types.SourceData{}
	if submission != nil {
		data = types.SourceData{
			SubmissionId: submission.SubmissionID,
			ProblemId:    submission.ProblemID,
			UserId:       submission.UserID,
			ContestId:    submission.ContestID,
			LanguageId:   submission.LanguageID,
			SourceCode:   submission.SourceCode,
			CreatedAt:    formatTime(submission.CreatedAt),
		}
	}
	return &types.GetSourceResponse{
		Code:    int(errors.Success),
		Message: "Success",
		Data:    data,
		TraceId: traceIDFromContext(ctx),
	}
}

func toJudgeStatusData(status domain.JudgeStatusPayload) types.JudgeStatusData {
	var compile *types.CompileResult
	if status.Compile != nil {
		compile = &types.CompileResult{
			OK:       status.Compile.OK,
			ExitCode: status.Compile.ExitCode,
			TimeMs:   status.Compile.TimeMs,
			MemoryKB: status.Compile.MemoryKB,
			Log:      status.Compile.Log,
			Error:    status.Compile.Error,
		}
	}
	tests := make([]types.TestcaseResult, 0, len(status.Tests))
	for _, test := range status.Tests {
		tests = append(tests, types.TestcaseResult{
			TestID:         test.TestID,
			Verdict:        test.Verdict,
			TimeMs:         test.TimeMs,
			MemoryKB:       test.MemoryKB,
			OutputKB:       test.OutputKB,
			ExitCode:       test.ExitCode,
			RuntimeLogPath: test.RuntimeLogPath,
			CheckerLogPath: test.CheckerLogPath,
			Stdout:         test.Stdout,
			Stderr:         test.Stderr,
			Score:          test.Score,
			SubtaskID:      test.SubtaskID,
		})
	}
	return types.JudgeStatusData{
		SubmissionId: status.SubmissionID,
		Status:       status.Status,
		Verdict:      status.Verdict,
		Score:        status.Score,
		Language:     status.Language,
		Summary: types.SummaryStat{
			TotalTimeMs:  status.Summary.TotalTimeMs,
			MaxMemoryKB:  status.Summary.MaxMemoryKB,
			TotalScore:   status.Summary.TotalScore,
			FailedTestID: status.Summary.FailedTestID,
		},
		Compile:      compile,
		Tests:        tests,
		Timestamps:   types.Timestamps{ReceivedAt: status.Timestamps.ReceivedAt, FinishedAt: status.Timestamps.FinishedAt},
		Progress:     types.Progress{TotalTests: status.Progress.TotalTests, DoneTests: status.Progress.DoneTests},
		ErrorCode:    status.ErrorCode,
		ErrorMessage: status.ErrorMessage,
	}
}

func traceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID := ctx.Value(contextkey.TraceID); traceID != nil {
		return fmt.Sprint(traceID)
	}
	return ""
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
