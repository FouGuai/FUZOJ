//go:build linux

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fuzoj/internal/judge/sandbox/spec"
)

func createRunCgroup(root, submissionID, testID string) (string, func(), error) {
	if root == "" {
		return "", func() {}, fmt.Errorf("cgroup root is required")
	}
	runDir := fmt.Sprintf("%s-%d", testID, time.Now().UnixNano())
	cgroupPath := filepath.Join(root, submissionID, runDir)
	if err := os.MkdirAll(cgroupPath, 0750); err != nil {
		return "", func() {}, fmt.Errorf("create cgroup path: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(cgroupPath)
	}
	return cgroupPath, cleanup, nil
}

func applyCgroupLimits(cgroupPath string, limits spec.ResourceLimit) error {
	pidsValue := "max"
	if limits.PIDs > 0 {
		pidsValue = strconv.FormatInt(limits.PIDs, 10)
	}
	if err := writeCgroupValue(cgroupPath, "pids.max", pidsValue); err != nil {
		return err
	}
	if limits.MemoryMB > 0 {
		if err := writeCgroupValue(cgroupPath, "memory.max", strconv.FormatInt(limits.MemoryMB*1024*1024, 10)); err != nil {
			return err
		}
	}
	if err := writeCgroupValue(cgroupPath, "cpu.max", "max 100000"); err != nil {
		return err
	}
	return nil
}

func addProcessToCgroup(cgroupPath string, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	return writeCgroupValue(cgroupPath, "cgroup.procs", strconv.Itoa(pid))
}

func killCgroup(cgroupPath string) error {
	killPath := filepath.Join(cgroupPath, "cgroup.kill")
	if _, err := os.Stat(killPath); err != nil {
		return err
	}
	return os.WriteFile(killPath, []byte("1"), 0600)
}

func wasOomKilled(cgroupPath string) bool {
	if cgroupPath == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(cgroupPath, "memory.events"))
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] == "oom_kill" {
			val, _ := strconv.ParseInt(fields[1], 10, 64)
			return val > 0
		}
	}
	return false
}

func memoryPeakKB(cgroupPath string, state *os.ProcessState) int64 {
	if cgroupPath != "" {
		if val, err := readCgroupInt(cgroupPath, "memory.peak"); err == nil && val > 0 {
			return val / 1024
		}
	}
	if state == nil {
		return 0
	}
	if usage, ok := state.SysUsage().(*syscall.Rusage); ok {
		return usage.Maxrss
	}
	return 0
}

func readCgroupInt(cgroupPath, name string) (int64, error) {
	data, err := os.ReadFile(filepath.Join(cgroupPath, name))
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(data))
	return strconv.ParseInt(value, 10, 64)
}

func writeCgroupValue(cgroupPath, name, value string) error {
	path := filepath.Join(cgroupPath, name)
	return os.WriteFile(path, []byte(value), 0640)
}
