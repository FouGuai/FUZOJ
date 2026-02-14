package profile

import "fuzoj/internal/judge/sandbox/spec"

// TaskType identifies the sandbox task category.
type TaskType string

const (
	TaskTypeCompile    TaskType = "compile"
	TaskTypeRun        TaskType = "run"
	TaskTypeChecker    TaskType = "checker"
	TaskTypeInteractor TaskType = "interactor"
	TaskTypeLint       TaskType = "lint"
)

// TaskProfile defines sandbox resources and security settings for a task type.
type TaskProfile struct {
	LanguageID     string
	TaskType       TaskType
	RootFS         string
	SeccompProfile string
	DefaultLimits  spec.ResourceLimit
}
