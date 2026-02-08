//go:build !linux

package engine

import (
	"context"
	"fmt"

	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/judge/sandbox/spec"
)

type stubEngine struct{}

func NewEngine(cfg Config, resolver ProfileResolver) (Engine, error) {
	return &stubEngine{}, nil
}

func (s *stubEngine) Run(ctx context.Context, runSpec spec.RunSpec) (result.RunResult, error) {
	return result.RunResult{}, fmt.Errorf("sandbox engine is only supported on linux")
}

func (s *stubEngine) KillSubmission(ctx context.Context, submissionID string) error {
	return fmt.Errorf("sandbox engine is only supported on linux")
}
