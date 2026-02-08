package runner

import (
	"context"

	"fuzoj/internal/judge/sandbox/profile"
	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/judge/sandbox/spec"
)

// CompileRequest describes one compilation task.
type CompileRequest struct {
	SubmissionID string
	Language     profile.LanguageSpec
	Profile      profile.TaskProfile
	RunSpec      spec.RunSpec
}

// RunRequest describes one execution task.
type RunRequest struct {
	SubmissionID string
	TestID       string
	Language     profile.LanguageSpec
	Profile      profile.TaskProfile
	RunSpec      spec.RunSpec
}

// Runner orchestrates compile and run workflows.
type Runner interface {
	Compile(ctx context.Context, req CompileRequest) (result.CompileResult, error)
	Run(ctx context.Context, req RunRequest) (result.TestcaseResult, error)
}
