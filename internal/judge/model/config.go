package model

import (
	"encoding/json"
	"fmt"
	"os"
)

// ProblemConfig defines judge-facing config fields.
type ProblemConfig struct {
	ProblemID      int64            `json:"problemId"`
	Version        int32            `json:"version"`
	Title          string           `json:"title"`
	DefaultLimits  ResourceLimit    `json:"defaultLimits"`
	LanguageLimits []LanguageLimits `json:"languageLimits"`
}

// LanguageLimits defines language-specific limits and compile flags.
type LanguageLimits struct {
	LanguageID        string         `json:"languageId"`
	ExtraCompileFlags []string       `json:"extraCompileFlags"`
	Limits            *ResourceLimit `json:"limits"`
}

// LoadProblemConfig parses config.json.
func LoadProblemConfig(path string) (ProblemConfig, error) {
	var cfg ProblemConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return ProblemConfig{}, fmt.Errorf("read config failed: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ProblemConfig{}, fmt.Errorf("parse config failed: %w", err)
	}
	return cfg, nil
}
