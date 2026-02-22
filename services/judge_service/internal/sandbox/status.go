// Package sandbox provides status reporting hooks for judge progress.
package sandbox

import (
	"context"

	"fuzoj/services/judge_service/internal/sandbox/result"
)

// StatusUpdate carries intermediate judge status data.
type StatusUpdate struct {
	SubmissionID string
	Status       result.JudgeStatus
	Language     string
	TotalTests   int
	DoneTests    int
	ReceivedAt   int64
	FinishedAt   int64
}

// StatusReporter persists intermediate status updates.
type StatusReporter interface {
	ReportStatus(ctx context.Context, update StatusUpdate) error
}
