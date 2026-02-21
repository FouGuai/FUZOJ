package config

import (
	"context"
	"fmt"

	"fuzoj/judge_service/internal/sandbox/profile"
	"fuzoj/judge_service/internal/sandbox/security"
	appErr "fuzoj/pkg/errors"
)

// LocalRepository loads language specs and task profiles from memory.
type LocalRepository struct {
	languages map[string]profile.LanguageSpec
	profiles  map[string]profile.TaskProfile
}

// NewLocalRepository creates a repository from config lists.
func NewLocalRepository(languages []profile.LanguageSpec, profiles []profile.TaskProfile) *LocalRepository {
	langMap := make(map[string]profile.LanguageSpec)
	for _, lang := range languages {
		if lang.ID == "" {
			continue
		}
		langMap[lang.ID] = lang
	}
	profileMap := make(map[string]profile.TaskProfile)
	for _, prof := range profiles {
		if prof.TaskType == "" || prof.LanguageID == "" {
			continue
		}
		key := profileName(prof.LanguageID, prof.TaskType)
		profileMap[key] = prof
	}
	return &LocalRepository{languages: langMap, profiles: profileMap}
}

// GetLanguageSpec returns a language spec.
func (r *LocalRepository) GetLanguageSpec(ctx context.Context, id string) (profile.LanguageSpec, error) {
	if id == "" {
		return profile.LanguageSpec{}, appErr.ValidationError("language_id", "required")
	}
	lang, ok := r.languages[id]
	if !ok {
		return profile.LanguageSpec{}, appErr.New(appErr.LanguageNotSupported).WithMessage("language not supported")
	}
	return lang, nil
}

// GetTaskProfile returns a task profile by type and language.
func (r *LocalRepository) GetTaskProfile(ctx context.Context, taskType profile.TaskType, languageID string) (profile.TaskProfile, error) {
	if taskType == "" || languageID == "" {
		return profile.TaskProfile{}, appErr.ValidationError("task_profile", "required")
	}
	key := fmt.Sprintf("%s-%s", languageID, taskType)
	prof, ok := r.profiles[key]
	if !ok {
		return profile.TaskProfile{}, appErr.New(appErr.NotFound).WithMessage("task profile not found")
	}
	return prof, nil
}

// Resolve maps a profile name to isolation settings.
func (r *LocalRepository) Resolve(profileName string) (security.IsolationProfile, error) {
	if profileName == "" {
		return security.IsolationProfile{}, appErr.ValidationError("profile", "required")
	}
	prof, ok := r.profiles[profileName]
	if !ok {
		return security.IsolationProfile{}, appErr.New(appErr.NotFound).WithMessage("profile not found")
	}
	return security.IsolationProfile{
		RootFS:         prof.RootFS,
		SeccompProfile: prof.SeccompProfile,
		DisableNetwork: true,
	}, nil
}

func profileName(languageID string, taskType profile.TaskType) string {
	return fmt.Sprintf("%s-%s", languageID, taskType)
}
