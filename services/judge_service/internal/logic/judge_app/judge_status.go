package judge_app

import (
	"context"
	"errors"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/sandbox"
	"fuzoj/services/judge_service/internal/sandbox/result"

	"github.com/zeromicro/go-zero/core/logx"
)

func (s *JudgeApp) persistStatus(ctx context.Context, status pmodel.JudgeStatusResponse) error {
	ctxStatus := ctx
	if s.statusTimeout > 0 {
		var cancel context.CancelFunc
		ctxStatus, cancel = context.WithTimeout(ctx, s.statusTimeout)
		defer cancel()
	}
	return s.statusRepo.Save(ctxStatus, status)
}

// ReportStatus updates intermediate judge status in cache.
func (s *JudgeApp) ReportStatus(ctx context.Context, update sandbox.StatusUpdate) error {
	status := pmodel.JudgeStatusResponse{
		SubmissionID: update.SubmissionID,
		Status:       update.Status,
		Language:     update.Language,
		Timestamps: result.Timestamps{
			ReceivedAt: update.ReceivedAt,
			FinishedAt: update.FinishedAt,
		},
		Progress: pmodel.Progress{
			TotalTests: update.TotalTests,
			DoneTests:  update.DoneTests,
		},
	}
	if err := s.persistStatus(ctx, status); err != nil {
		logx.WithContext(ctx).Errorf("update intermediate status failed: %v", err)
		return err
	}
	return nil
}

func (s *JudgeApp) handleFailure(ctx context.Context, submissionID string, err error) error {
	code := appErr.GetCode(err)
	failed := pmodel.JudgeStatusResponse{
		SubmissionID: submissionID,
		Status:       result.StatusFailed,
		Verdict:      result.VerdictSE,
		ErrorCode:    int(code),
		ErrorMessage: err.Error(),
		Timestamps: result.Timestamps{
			FinishedAt: time.Now().Unix(),
		},
	}
	if saveErr := s.persistStatus(ctx, failed); saveErr != nil {
		logx.WithContext(ctx).Errorf("update failure status failed: %v", saveErr)
	}
	if code == appErr.InvalidParams || code == appErr.ProblemNotFound || code == appErr.LanguageNotSupported {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return err
}
