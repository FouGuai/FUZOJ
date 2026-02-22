package pmodel

import (
	"encoding/json"
	"fmt"
	"os"
)

// Manifest defines the testcase bundle layout.
type Manifest struct {
	ProblemID int64             `json:"problemId"`
	Version   int32             `json:"version"`
	IOConfig  ManifestIOConfig  `json:"ioConfig"`
	Checker   *CheckerSpec      `json:"checker"`
	Tests     []ManifestTest    `json:"tests"`
	Subtasks  []ManifestSubtask `json:"subtasks"`
	Hash      ManifestHash      `json:"hash"`
}

// ManifestIOConfig defines IO mode and file names.
type ManifestIOConfig struct {
	Mode           string `json:"mode"`
	InputFileName  string `json:"inputFileName"`
	OutputFileName string `json:"outputFileName"`
}

// CheckerSpec describes checker binary and limits.
type CheckerSpec struct {
	BinaryPath string         `json:"binaryPath"`
	Args       []string       `json:"args"`
	Env        []string       `json:"env"`
	Limits     *ResourceLimit `json:"limits"`
}

// ManifestTest describes one testcase.
type ManifestTest struct {
	TestID            string         `json:"testId"`
	InputPath         string         `json:"inputPath"`
	AnswerPath        string         `json:"answerPath"`
	Score             int            `json:"score"`
	SubtaskID         string         `json:"subtaskId"`
	Limits            *ResourceLimit `json:"limits"`
	Checker           *CheckerSpec   `json:"checker"`
	CheckerLanguageID string         `json:"checkerLanguageId"`
}

// ManifestSubtask defines scoring group.
type ManifestSubtask struct {
	ID         string `json:"id"`
	Score      int    `json:"score"`
	Strategy   string `json:"strategy"`
	StopOnFail bool   `json:"stopOnFail"`
}

// ManifestHash stores bundle hashes.
type ManifestHash struct {
	ManifestHash string `json:"manifestHash"`
	DataPackHash string `json:"dataPackHash"`
}

// ResourceLimit matches sandbox limits.
type ResourceLimit struct {
	CPUTimeMs  int64 `json:"timeMs"`
	WallTimeMs int64 `json:"wallTimeMs"`
	MemoryMB   int64 `json:"memoryMB"`
	StackMB    int64 `json:"stackMB"`
	OutputMB   int64 `json:"outputMB"`
	PIDs       int64 `json:"processes"`
}

// LoadManifest parses manifest.json.
func LoadManifest(path string) (Manifest, error) {
	var m Manifest
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest failed: %w", err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest failed: %w", err)
	}
	return m, nil
}
