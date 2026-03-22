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

	"github.com/zeromicro/go-zero/core/logx"
)

func createRunCgroup(root, submissionID, testID string) (string, func(), error) {
	if root == "" {
		return "", func() {}, appErr.ValidationError("cgroup_root", "required")
	}
	if err := enableSubtreeControllers(root, []string{"cpu", "memory", "pids"}); err != nil {
		return "", func() {}, appErr.Wrapf(err, appErr.JudgeSystemError, "enable root cgroup controllers failed")
	}
	submissionPath := filepath.Join(root, submissionID)
	if err := os.MkdirAll(submissionPath, 0750); err != nil {
		return "", func() {}, appErr.Wrapf(err, appErr.JudgeSystemError, "create submission cgroup path failed")
	}
	if err := enableSubtreeControllers(submissionPath, []string{"cpu", "memory", "pids"}); err != nil {
		return "", func() {}, appErr.Wrapf(err, appErr.JudgeSystemError, "enable submission cgroup controllers failed")
	}
	runDir := fmt.Sprintf("%s-%d", testID, time.Now().UnixNano())
	cgroupPath := filepath.Join(submissionPath, runDir)
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
		logx.Errorf("write pids.max failed: cgroupPath=%s value=%s err=%v", cgroupPath, pidsValue, err)
		return appErr.Wrapf(err, appErr.JudgeSystemError, "write pids.max failed")
	}
	if limits.MemoryMB > 0 {
		if err := writeCgroupValue(cgroupPath, "memory.max", strconv.FormatInt(limits.MemoryMB*1024*1024, 10)); err != nil {
			logx.Errorf("write memory.max failed: cgroupPath=%s valueMB=%d err=%v", cgroupPath, limits.MemoryMB, err)
			return appErr.Wrapf(err, appErr.JudgeSystemError, "write memory.max failed")
		}
	}
	if err := writeCgroupValue(cgroupPath, "cpu.max", "max 100000"); err != nil {
		logx.Errorf("write cpu.max failed: cgroupPath=%s err=%v", cgroupPath, err)
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

func memoryPeakKB(cgroupPath string, state *os.ProcessState, sampledPeakBytes int64) int64 {
	sampledPeakKB := int64(0)
	if sampledPeakBytes > 0 {
		sampledPeakKB = sampledPeakBytes / 1024
	}
	if cgroupPath != "" {
		if val, err := readCgroupInt(cgroupPath, "memory.peak"); err == nil && val > 0 {
			return maxInt64(val/1024, sampledPeakKB)
		}
		if current, err := cgroupMemoryCurrentBytes(cgroupPath); err == nil && current > 0 {
			return maxInt64(current/1024, sampledPeakKB)
		}
	}
	if state == nil {
		return sampledPeakKB
	}
	if usage, ok := state.SysUsage().(*syscall.Rusage); ok {
		return maxInt64(usage.Maxrss, sampledPeakKB)
	}
	return sampledPeakKB
}

func cgroupMemoryCurrentBytes(cgroupPath string) (int64, error) {
	if cgroupPath == "" {
		return 0, appErr.ValidationError("cgroup_path", "required")
	}
	data, err := os.ReadFile(filepath.Join(cgroupPath, "memory.current"))
	if err != nil {
		return 0, appErr.Wrapf(err, appErr.JudgeSystemError, "read memory.current failed")
	}
	value := strings.TrimSpace(string(data))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, appErr.Wrapf(err, appErr.JudgeSystemError, "parse memory.current failed")
	}
	return parsed, nil
}

func maxInt64(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
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
		logx.Errorf("write cgroup value failed: path=%s value=%s err=%v", path, value, err)
		return appErr.Wrapf(err, appErr.JudgeSystemError, "write cgroup value failed")
	}
	return nil
}

func enableSubtreeControllers(cgroupPath string, controllers []string) error {
	ctrlPath := filepath.Join(cgroupPath, "cgroup.subtree_control")
	if _, err := os.Stat(ctrlPath); err != nil {
		return nil
	}
	currentRaw, err := os.ReadFile(ctrlPath)
	if err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "read cgroup.subtree_control failed")
	}
	current := strings.Fields(strings.TrimSpace(string(currentRaw)))
	missing := make([]string, 0, len(controllers))
	for _, name := range controllers {
		exists := false
		for _, enabled := range current {
			if enabled == name {
				exists = true
				break
			}
		}
		if !exists {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	value := strings.Join(func(names []string) []string {
		prefixed := make([]string, 0, len(names))
		for _, name := range names {
			prefixed = append(prefixed, "+"+name)
		}
		return prefixed
	}(missing), " ")
	if err := os.WriteFile(ctrlPath, []byte(value), 0640); err != nil {
		logx.Errorf("write cgroup.subtree_control failed: path=%s value=%s err=%v", ctrlPath, value, err)
		return appErr.Wrapf(err, appErr.JudgeSystemError, "write cgroup.subtree_control failed")
	}
	return nil
}
