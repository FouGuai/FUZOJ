package service

import (
	"context"
	"errors"
	"time"

	"fuzoj/internal/judge/model"
	"fuzoj/internal/judge/sandbox"
	"fuzoj/internal/judge/sandbox/result"
	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

func (s *Service) persistStatus(ctx context.Context, status model.JudgeStatusResponse) error {
	ctxStatus := ctx
	if s.statusTimeout > 0 {
		var cancel context.CancelFunc
		ctxStatus, cancel = context.WithTimeout(ctx, s.statusTimeout)
		defer cancel()
	}
	return s.statusRepo.Save(ctxStatus, status)
}

// ReportStatus updates intermediate judge status in cache.
func (s *Service) ReportStatus(ctx context.Context, update sandbox.StatusUpdate) error {
	status := model.JudgeStatusResponse{
		SubmissionID: update.SubmissionID,
		Status:       update.Status,
		Language:     update.Language,
		Timestamps: result.Timestamps{
			ReceivedAt: update.ReceivedAt,
			FinishedAt: update.FinishedAt,
		},
		Progress: model.Progress{
			TotalTests: update.TotalTests,
			DoneTests:  update.DoneTests,
		},
	}
	if err := s.persistStatus(ctx, status); err != nil {
		logger.Warn(ctx, "update intermediate status failed", zap.Error(err))
		return err
	}
	return nil
}

func (s *Service) handleFailure(ctx context.Context, submissionID string, err error) error {
	code := appErr.GetCode(err)
	failed := model.JudgeStatusResponse{
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
		logger.Warn(ctx, "update failure status failed", zap.Error(saveErr))
	}
	if code == appErr.InvalidParams || code == appErr.ProblemNotFound || code == appErr.LanguageNotSupported {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return err
}
