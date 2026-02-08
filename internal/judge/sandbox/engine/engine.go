package engine

import (
	"context"

	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/judge/sandbox/spec"
)

// Engine executes a RunSpec inside an isolated sandbox.
type Engine interface {
	Run(ctx context.Context, runSpec spec.RunSpec) (result.RunResult, error)
	KillSubmission(ctx context.Context, submissionID string) error
}
