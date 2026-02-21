// Package sandbox defines the public call interface used by the judge service.
package sandbox

import (
	"context"

	"fuzoj/judge_service/internal/sandbox/result"
	"fuzoj/judge_service/internal/sandbox/spec"
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
	LanguageID   string
	WorkRoot     string
	SourcePath   string

	Tests    []TestcaseSpec
	Subtasks []SubtaskSpec

	// ExtraCompileFlags must be filtered by the caller before use.
	ExtraCompileFlags []string

	// Business metadata.
	ProblemID string
	ContestID string
	UserID    string
	Priority  int
	Tags      []string

	// ReceivedAt is the unix timestamp when the judge task was accepted.
	ReceivedAt int64
}

// TestcaseSpec describes one test case input and expected answer.
type TestcaseSpec struct {
	TestID     string
	InputPath  string
	AnswerPath string
	IOConfig   IOConfig
	Score      int
	SubtaskID  string
	Limits     spec.ResourceLimit
	Checker    *CheckerSpec
	// CheckerLanguageID defaults to LanguageID if empty.
	CheckerLanguageID string
}

// SubtaskSpec defines scoring strategy for a group of testcases.
type SubtaskSpec struct {
	ID         string
	Score      int
	Strategy   string
	StopOnFail bool
}

// CheckerSpec describes the special judge binary and its arguments.
type CheckerSpec struct {
	BinaryPath string
	Args       []string
	Env        []string
	Limits     spec.ResourceLimit
}
