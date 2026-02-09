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
	WorkDir      string
	SourcePath   string
	// ExtraCompileFlags must be pre-filtered by the caller.
	ExtraCompileFlags []string
	Limits            spec.ResourceLimit
}

// RunRequest describes one execution task.
type RunRequest struct {
	SubmissionID   string
	TestID         string
	Language       profile.LanguageSpec
	Profile        profile.TaskProfile
	WorkDir        string
	IOConfig       IOConfig
	InputPath      string
	AnswerPath     string
	Limits         spec.ResourceLimit
	Checker        *CheckerSpec
	CheckerProfile *profile.TaskProfile
	Score          int
	SubtaskID      string
}

// Runner orchestrates compile and run workflows.
type Runner interface {
	Compile(ctx context.Context, req CompileRequest) (result.CompileResult, error)
	Run(ctx context.Context, req RunRequest) (result.TestcaseResult, error)
}

// CppCompileRequest extends CompileRequest for C++ tasks.
type CppCompileRequest struct {
	CompileRequest
}

// CppRunRequest extends RunRequest for C++ tasks.
type CppRunRequest struct {
	RunRequest
}

// IOConfig describes how the program reads input and writes output.
type IOConfig struct {
	Mode           string
	InputFileName  string
	OutputFileName string
}

// CheckerSpec describes the special judge binary and its arguments.
type CheckerSpec struct {
	BinaryPath string
	Args       []string
	Env        []string
	Limits     spec.ResourceLimit
}
