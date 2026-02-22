//go:build linux

package engine

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"fuzoj/services/judge_service/internal/sandbox/spec"
)

func durationFromMs(ms int64) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func cpuTimeMs(state *os.ProcessState) int64 {
	if state == nil {
		return 0
	}
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	utime := time.Duration(usage.Utime.Sec)*time.Second + time.Duration(usage.Utime.Usec)*time.Microsecond
	stime := time.Duration(usage.Stime.Sec)*time.Second + time.Duration(usage.Stime.Usec)*time.Microsecond
	return int64((utime + stime).Milliseconds())
}

func stdoutSizeKB(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size() / 1024
}

func readLimitedFile(path string, maxBytes int64) string {
	if path == "" || maxBytes <= 0 {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	limited := io.LimitReader(file, maxBytes)
	data, err := io.ReadAll(limited)
	if err != nil {
		return ""
	}
	return string(data)
}

func resolveHostPath(path string, runSpec spec.RunSpec) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	longest := ""
	source := ""
	for _, mount := range runSpec.BindMounts {
		if mount.Target == "" || mount.Source == "" {
			continue
		}
		target := filepath.Clean(mount.Target)
		if !strings.HasPrefix(clean, target) {
			continue
		}
		if len(target) > len(longest) {
			longest = target
			source = mount.Source
		}
	}
	if source == "" {
		return path
	}
	rel := strings.TrimPrefix(clean, longest)
	rel = strings.TrimPrefix(rel, string(os.PathSeparator))
	return filepath.Join(source, rel)
}
