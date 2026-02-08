// Package sandbox defines the public call interface used by the judge service.
package sandbox

import (
	"context"

	"fuzoj/internal/judge/sandbox/profile"
	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/judge/sandbox/spec"
)

// Service is the high-level sandbox entrypoint used by the judge layer.
type Service interface {
	Judge(ctx context.Context, req JudgeRequest) (result.JudgeResult, error)
	Kill(ctx context.Context, submissionID string) error
}

// IOConfig describes how the program reads input and writes output.
type IOConfig struct {
	// Mode is "stdio" or "fileio".
	Mode string
	// InputFileName is required when Mode is "fileio".
	InputFileName string
	// OutputFileName is required when Mode is "fileio".
	OutputFileName string
}

// JudgeRequest contains all data needed to execute one submission.
// All paths must point to local files prepared before calling the sandbox.
type JudgeRequest struct {
	SubmissionID string
	Language     profile.LanguageSpec

	// CompileProfile is required when Language.CompileEnabled is true.
	// RunProfile is always required.
	CompileProfile profile.TaskProfile
	RunProfile     profile.TaskProfile

	// Optional profiles for special tasks.
	CheckerProfile    *profile.TaskProfile
	InteractorProfile *profile.TaskProfile

	// WorkRoot is the host path used to create per-test workspaces.
	WorkRoot string
	// SourcePath is the local path to the user source code.
	SourcePath string

	IOConfig   IOConfig
	Tests      []TestcaseSpec
	Checker    *CheckerSpec
	Interactor *InteractorSpec

	// ExtraCompileFlags must be filtered by the caller before use.
	ExtraCompileFlags []string
}

// TestcaseSpec describes one test case input and expected answer.
type TestcaseSpec struct {
	TestID     string
	InputPath  string
	AnswerPath string
	Score      int
	SubtaskID  string
	Limits     spec.ResourceLimit
}

// CheckerSpec describes the special judge binary and its arguments.
type CheckerSpec struct {
	BinaryPath string
	Args       []string
	Env        []string
	Limits     spec.ResourceLimit
}

// InteractorSpec describes the interactor binary and its arguments.
type InteractorSpec struct {
	BinaryPath string
	Args       []string
	Env        []string
	Limits     spec.ResourceLimit
}
