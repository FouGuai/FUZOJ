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

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/sandbox/spec"
)

func createRunCgroup(root, submissionID, testID string) (string, func(), error) {
	if root == "" {
		return "", func() {}, appErr.ValidationError("cgroup_root", "required")
	}
	runDir := fmt.Sprintf("%s-%d", testID, time.Now().UnixNano())
	cgroupPath := filepath.Join(root, submissionID, runDir)
	if err := os.MkdirAll(cgroupPath, 0750); err != nil {
		return "", func() {}, appErr.Wrapf(err, appErr.JudgeSystemError, "create cgroup path failed")
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
		return appErr.Wrapf(err, appErr.JudgeSystemError, "write pids.max failed")
	}
	if limits.MemoryMB > 0 {
		if err := writeCgroupValue(cgroupPath, "memory.max", strconv.FormatInt(limits.MemoryMB*1024*1024, 10)); err != nil {
			return appErr.Wrapf(err, appErr.JudgeSystemError, "write memory.max failed")
		}
	}
	if err := writeCgroupValue(cgroupPath, "cpu.max", "max 100000"); err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "write cpu.max failed")
	}
	return nil
}

func addProcessToCgroup(cgroupPath string, pid int) error {
	if pid <= 0 {
		return appErr.ValidationError("pid", "invalid")
	}
	if err := writeCgroupValue(cgroupPath, "cgroup.procs", strconv.Itoa(pid)); err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "write cgroup.procs failed")
	}
	return nil
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

func cgroupCPUTimeMs(cgroupPath string) (int64, error) {
	if cgroupPath == "" {
		return 0, appErr.ValidationError("cgroup_path", "required")
	}
	data, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.stat"))
	if err != nil {
		return 0, appErr.Wrapf(err, appErr.JudgeSystemError, "read cpu.stat failed")
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] == "usage_usec" {
			val, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, appErr.Wrapf(err, appErr.JudgeSystemError, "parse cpu.stat usage_usec failed")
			}
			return val / 1000, nil
		}
	}
	return 0, appErr.New(appErr.JudgeSystemError).WithMessage("usage_usec not found in cpu.stat")
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
		return 0, appErr.Wrapf(err, appErr.JudgeSystemError, "read cgroup value failed")
	}
	value := strings.TrimSpace(string(data))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, appErr.Wrapf(err, appErr.JudgeSystemError, "parse cgroup value failed")
	}
	return parsed, nil
}

func writeCgroupValue(cgroupPath, name, value string) error {
	path := filepath.Join(cgroupPath, name)
	if err := os.WriteFile(path, []byte(value), 0640); err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "write cgroup value failed")
	}
	return nil
}
