package consumer

import (
	"context"

	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/repository"
)

// DispatchDoneHandler marks dispatch record done when final status is consumed.
type DispatchDoneHandler struct {
	repo repository.SubmissionDispatchRepository
}

// NewDispatchDoneHandler creates a final-status handler for dispatch outbox.
func NewDispatchDoneHandler(repo repository.SubmissionDispatchRepository) *DispatchDoneHandler {
	if repo == nil {
		return nil
	}
	return &DispatchDoneHandler{repo: repo}
}

// HandleFinalStatus marks submission dispatch as done.
func (h *DispatchDoneHandler) HandleFinalStatus(ctx context.Context, status domain.JudgeStatusPayload) error {
	if h == nil || h.repo == nil {
		return nil
	}
	return h.repo.MarkDone(ctx, nil, status.SubmissionID)
}
