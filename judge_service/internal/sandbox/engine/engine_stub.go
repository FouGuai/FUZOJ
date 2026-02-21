//go:build !linux

package engine

import (
	"context"

	"fuzoj/judge_service/internal/sandbox/result"
	"fuzoj/judge_service/internal/sandbox/spec"
	appErr "fuzoj/pkg/errors"
)

type stubEngine struct{}

func NewEngine(cfg Config, resolver ProfileResolver) (Engine, error) {
	return &stubEngine{}, nil
}

func (s *stubEngine) Run(ctx context.Context, runSpec spec.RunSpec) (result.RunResult, error) {
	return result.RunResult{}, appErr.New(appErr.JudgeSystemError).WithMessage("sandbox engine is only supported on linux")
}

func (s *stubEngine) KillSubmission(ctx context.Context, submissionID string) error {
	return appErr.New(appErr.JudgeSystemError).WithMessage("sandbox engine is only supported on linux")
}
