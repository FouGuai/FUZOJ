// Package config defines interfaces for loading sandbox configuration.
package config

import (
	"context"

	"fuzoj/judge_service/internal/sandbox/profile"
)

// LanguageSpecRepository loads language specifications.
type LanguageSpecRepository interface {
	GetLanguageSpec(ctx context.Context, id string) (profile.LanguageSpec, error)
}

// TaskProfileRepository loads task profiles by type and language.
type TaskProfileRepository interface {
	GetTaskProfile(ctx context.Context, taskType profile.TaskType, languageID string) (profile.TaskProfile, error)
}
