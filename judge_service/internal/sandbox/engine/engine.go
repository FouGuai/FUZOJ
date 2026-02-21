package engine

import (
	"context"

	"fuzoj/judge_service/internal/sandbox/result"
	"fuzoj/judge_service/internal/sandbox/spec"
)

// Engine executes a RunSpec inside an isolated sandbox.
type Engine interface {
	Run(ctx context.Context, runSpec spec.RunSpec) (result.RunResult, error)
	KillSubmission(ctx context.Context, submissionID string) error
}
