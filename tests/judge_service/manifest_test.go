package judge_service_test

import (
	"os"
	"path/filepath"
	"testing"

	"fuzoj/judge_service/internal/model"
)

func TestMergeLimits(t *testing.T) {
	defaults := model.ResourceLimit{CPUTimeMs: 1000, WallTimeMs: 2000, MemoryMB: 256, StackMB: 64, OutputMB: 32, PIDs: 16}
	override := &model.ResourceLimit{CPUTimeMs: 5000, MemoryMB: 512}
	merged := model.MergeLimits(override, defaults)
	if merged.CPUTimeMs != 5000 {
		t.Fatalf("expected cpu time override")
	}
	if merged.WallTimeMs != 2000 {
		t.Fatalf("expected wall time default")
	}
	if merged.MemoryMB != 512 {
		t.Fatalf("expected memory override")
	}
	if merged.StackMB != 64 {
		t.Fatalf("expected stack default")
	}
}

func TestLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "manifest.json")
	data := []byte(`{"problemId":1,"version":1,"ioConfig":{"mode":"stdio"},"tests":[{"testId":"1","inputPath":"data/1.in","answerPath":"data/1.ans","score":10}]}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	m, err := model.LoadManifest(path)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if m.ProblemID != 1 || m.Version != 1 {
		t.Fatalf("unexpected manifest meta")
	}
	if len(m.Tests) != 1 || m.Tests[0].TestID != "1" {
		t.Fatalf("unexpected tests")
	}
}
