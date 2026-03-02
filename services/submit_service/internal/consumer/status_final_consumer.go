package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/repository"
)

// FinalStatusHandler handles final status events for post-processing.
type FinalStatusHandler interface {
	HandleFinalStatus(ctx context.Context, status domain.JudgeStatusPayload) error
}

// TimeoutConfig holds timeout settings for external calls used by consumer.
type TimeoutConfig struct {
	DB time.Duration
}

// StatusFinalConsumer processes final status messages from MQ.
type StatusFinalConsumer struct {
	statusRepo *repository.StatusRepository
	logRepo    *repository.SubmissionLogRepository
	handlers   []FinalStatusHandler
	timeouts   TimeoutConfig
}

// NewStatusFinalConsumer creates a consumer for final status events.
func NewStatusFinalConsumer(statusRepo *repository.StatusRepository, logRepo *repository.SubmissionLogRepository, handlers []FinalStatusHandler, timeouts TimeoutConfig) *StatusFinalConsumer {
	return &StatusFinalConsumer{
		statusRepo: statusRepo,
		logRepo:    logRepo,
		handlers:   handlers,
		timeouts:   timeouts,
	}
}

// Consume processes a final status message.
func (c *StatusFinalConsumer) Consume(ctx context.Context, key, value string) error {
	if c == nil || c.statusRepo == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("status repository is not configured")
	}
	if value == "" {
		return nil
	}
	var event domain.StatusEvent
	if err := json.Unmarshal([]byte(value), &event); err != nil {
		return appErr.Wrapf(err, appErr.InvalidParams, "decode status event failed")
	}
	if event.Type != domain.StatusEventFinal {
		return appErr.New(appErr.InvalidParams).WithMessage("status event type is invalid")
	}
	if event.Status.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if err := c.persistFinalStatus(ctx, event.Status); err != nil {
		return err
	}
	for _, handler := range c.handlers {
		if handler == nil {
			continue
		}
		if err := handler.HandleFinalStatus(ctx, event.Status); err != nil {
			return fmt.Errorf("handle final status failed: %w", err)
		}
	}
	return nil
}

func (c *StatusFinalConsumer) persistFinalStatus(ctx context.Context, status domain.JudgeStatusPayload) error {
	ctxDB := withTimeout(ctx, c.timeouts.DB)
	defer ctxDB.cancel()
	cleanStatus := status
	if c.logRepo != nil {
		logs := []repository.LogRecord{}
		cleanStatus, logs = repository.ExtractLogs(status)
		if len(logs) > 0 {
			if err := c.logRepo.SaveBatch(ctxDB.ctx, logs); err != nil {
				return err
			}
		}
	}
	if err := c.statusRepo.Save(ctxDB.ctx, cleanStatus); err != nil {
		return err
	}
	return c.statusRepo.PersistFinalStatus(ctxDB.ctx, cleanStatus)
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
