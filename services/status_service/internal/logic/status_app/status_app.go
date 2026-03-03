package status_app

import (
	"context"
	"strings"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/status_service/internal/domain"
	"fuzoj/services/status_service/internal/repository"
	"fuzoj/services/status_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type TimeoutConfig struct {
	DB      time.Duration
	Cache   time.Duration
	Storage time.Duration
	Status  time.Duration
}

// StatusApp provides status query operations.
type StatusApp struct {
	statusRepo *repository.StatusRepository
	logRepo    *repository.SubmissionLogRepository
	timeouts   TimeoutConfig
}

func NewStatusApp(svcCtx *svc.ServiceContext) (*StatusApp, error) {
	if svcCtx == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("status app is not configured")
	}
	if svcCtx.StatusRepo == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("status repository is not configured")
	}
	return &StatusApp{
		statusRepo: svcCtx.StatusRepo,
		logRepo:    svcCtx.LogRepo,
		timeouts: TimeoutConfig{
			DB:      svcCtx.Config.Status.Timeouts.DB,
			Cache:   svcCtx.Config.Status.Timeouts.Cache,
			Storage: svcCtx.Config.Status.Timeouts.Storage,
			Status:  svcCtx.Config.Status.Timeouts.Status,
		},
	}, nil
}

func (a *StatusApp) GetStatus(ctx context.Context, submissionID, include string) (domain.JudgeStatusPayload, error) {
	if submissionID == "" {
		return domain.JudgeStatusPayload{}, appErr.ValidationError("submission_id", "required")
	}
	ctxStatus := withTimeout(ctx, a.timeouts.Status)
	defer ctxStatus.cancel()

	status, err := a.statusRepo.Get(ctxStatus.ctx, submissionID)
	if err != nil {
		return domain.JudgeStatusPayload{}, err
	}
	if !isFinalStatus(status.Status) || strings.TrimSpace(include) == "" {
		return summaryStatus(status), nil
	}
	finalStatus, err := a.statusRepo.GetFinalDetail(ctxStatus.ctx, submissionID)
	if err != nil {
		return domain.JudgeStatusPayload{}, err
	}
	if a.logRepo == nil {
		logx.WithContext(ctx).Error("log repository is not configured")
		return finalStatus, nil
	}
	return a.withLogs(ctxStatus.ctx, finalStatus)
}

func (a *StatusApp) withLogs(ctx context.Context, status domain.JudgeStatusPayload) (domain.JudgeStatusPayload, error) {
	if status.Compile != nil {
		if status.Compile.Log == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeCompileLog, ""); err == nil {
				status.Compile.Log = logItem.Content
			}
		}
		if status.Compile.Error == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeCompileError, ""); err == nil {
				status.Compile.Error = logItem.Content
			}
		}
	}
	if len(status.Tests) == 0 {
		return status, nil
	}
	tests := make([]domain.TestcaseResult, 0, len(status.Tests))
	for _, test := range status.Tests {
		item := test
		if item.RuntimeLog == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeRuntime, item.TestID); err == nil {
				item.RuntimeLog = logItem.Content
			}
		}
		if item.CheckerLog == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeChecker, item.TestID); err == nil {
				item.CheckerLog = logItem.Content
			}
		}
		if item.Stdout == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeStdout, item.TestID); err == nil {
				item.Stdout = logItem.Content
			}
		}
		if item.Stderr == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeStderr, item.TestID); err == nil {
				item.Stderr = logItem.Content
			}
		}
		tests = append(tests, item)
	}
	status.Tests = tests
	return status, nil
}

func isFinalStatus(status string) bool {
	return status == domain.StatusFinished || status == domain.StatusFailed
}

func summaryStatus(status domain.JudgeStatusPayload) domain.JudgeStatusPayload {
	summary := status
	summary.Compile = nil
	summary.Tests = nil
	return summary
}

type timeoutCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func withTimeout(ctx context.Context, timeout time.Duration) timeoutCtx {
	if timeout <= 0 {
		return timeoutCtx{ctx: ctx, cancel: func() {}}
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx{ctx: ctxTimeout, cancel: cancel}
}
