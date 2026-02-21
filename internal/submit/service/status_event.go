package service

import (
	"context"
	"encoding/json"
	"fmt"

	"fuzoj/internal/common/mq"
	"fuzoj/judge_service/internal/model"
	appErr "fuzoj/pkg/errors"
)

// FinalStatusHandler handles final status events for post-processing.
type FinalStatusHandler interface {
	HandleFinalStatus(ctx context.Context, status model.JudgeStatusResponse) error
}

// HandleFinalStatusMessage processes final status messages from MQ.
func (s *SubmitService) HandleFinalStatusMessage(ctx context.Context, msg *mq.Message) error {
	if msg == nil {
		return appErr.New(appErr.InvalidParams).WithMessage("message is nil")
	}
	var event model.StatusEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return appErr.Wrapf(err, appErr.InvalidParams, "decode status event failed")
	}
	if event.Type != model.StatusEventFinal {
		return appErr.New(appErr.InvalidParams).WithMessage("status event type is invalid")
	}
	if event.Status.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if err := s.persistFinalStatus(ctx, event.Status); err != nil {
		return err
	}
	for _, handler := range s.finalStatusHandlers {
		if handler == nil {
			continue
		}
		if err := handler.HandleFinalStatus(ctx, event.Status); err != nil {
			return fmt.Errorf("handle final status failed: %w", err)
		}
	}
	return nil
}

func (s *SubmitService) persistFinalStatus(ctx context.Context, status model.JudgeStatusResponse) error {
	ctxDB := withTimeout(ctx, s.timeouts.DB)
	defer ctxDB.cancel()
	return s.statusRepo.PersistFinalStatus(ctxDB.ctx, status)
}
