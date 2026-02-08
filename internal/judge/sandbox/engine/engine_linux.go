//go:build linux

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/judge/sandbox/security"
	"fuzoj/internal/judge/sandbox/spec"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

const (
	defaultStdoutStderrMaxBytes int64 = 64 * 1024
)

type linuxEngine struct {
	cfg       Config
	resolver  ProfileResolver
	registry  map[string][]string
	registryM sync.Mutex
}

// NewEngine creates a Linux sandbox engine.
func NewEngine(cfg Config, resolver ProfileResolver) (Engine, error) {
	if resolver == nil {
		return nil, fmt.Errorf("profile resolver is required")
	}
	if cfg.StdoutStderrMaxBytes <= 0 {
		cfg.StdoutStderrMaxBytes = defaultStdoutStderrMaxBytes
	}
	if cfg.HelperPath == "" {
		cfg.HelperPath = "sandbox-init"
	}
	return &linuxEngine{
		cfg:      cfg,
		resolver: resolver,
		registry: make(map[string][]string),
	}, nil
}

func (e *linuxEngine) Run(ctx context.Context, runSpec spec.RunSpec) (result.RunResult, error) {
	if err := validateRunSpec(runSpec); err != nil {
		return result.RunResult{}, err
	}

	isoProfile, err := e.resolver.Resolve(runSpec.Profile)
	if err != nil {
		return result.RunResult{}, fmt.Errorf("resolve profile: %w", err)
	}
	if e.cfg.SeccompDir != "" && isoProfile.SeccompProfile != "" && !filepath.IsAbs(isoProfile.SeccompProfile) {
		isoProfile.SeccompProfile = filepath.Join(e.cfg.SeccompDir, isoProfile.SeccompProfile)
	}

	cgroupPath := ""
	cgroupCleanup := func() {}
	if e.cfg.EnableCgroup {
		cgroupPath, cgroupCleanup, err = createRunCgroup(e.cfg.CgroupRoot, runSpec.SubmissionID, runSpec.TestID)
		if err != nil {
			return result.RunResult{}, fmt.Errorf("create cgroup: %w", err)
		}
		if err := applyCgroupLimits(cgroupPath, runSpec.Limits); err != nil {
			cgroupCleanup()
			return result.RunResult{}, fmt.Errorf("apply cgroup limits: %w", err)
		}
		e.registerCgroup(runSpec.SubmissionID, cgroupPath)
	}
	defer func() {
		if e.cfg.EnableCgroup {
			e.unregisterCgroup(runSpec.SubmissionID, cgroupPath)
			cgroupCleanup()
		}
	}()

	initReq := initRequest{
		RunSpec:       runSpec,
		Isolation:     isoProfile,
		EnableSeccomp: e.cfg.EnableSeccomp,
		EnableNs:      e.cfg.EnableNamespaces,
	}

	stdinPipe, err := jsonToPipe(initReq)
	if err != nil {
		return result.RunResult{}, fmt.Errorf("encode init request: %w", err)
	}
	defer stdinPipe.Close()

	cmd := exec.CommandContext(ctx, e.cfg.HelperPath)
	cmd.SysProcAttr = buildSysProcAttr(isoProfile, e.cfg.EnableNamespaces)
	cmd.Stdin = stdinPipe

	var helperStdout bytes.Buffer
	var helperStderr bytes.Buffer
	cmd.Stdout = &helperStdout
	cmd.Stderr = &helperStderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return result.RunResult{}, fmt.Errorf("start helper: %w", err)
	}

	if e.cfg.EnableCgroup {
		if err := addProcessToCgroup(cgroupPath, cmd.Process.Pid); err != nil {
			logger.Warn(ctx, "add process to cgroup failed", zap.String("cgroup", cgroupPath), zap.Error(err))
		}
	}

	var timedOut atomic.Bool
	killCtx, cancelKill := context.WithCancel(ctx)
	defer cancelKill()

	done := make(chan struct{})
	go func() {
		wallLimit := durationFromMs(runSpec.Limits.WallTimeMs)
		var wallTimer <-chan time.Time
		if wallLimit > 0 {
			wallTimer = time.After(wallLimit)
		}
		select {
		case <-killCtx.Done():
			e.killProcessGroup(cmd.Process.Pid)
		case <-wallTimer:
			timedOut.Store(true)
			e.killProcessGroup(cmd.Process.Pid)
		case <-done:
		}
	}()

	waitErr := cmd.Wait()
	close(done)

	if waitErr != nil {
		if helperStderr.Len() > 0 {
			logger.Warn(ctx, "helper stderr", zap.String("stderr", helperStderr.String()))
		}
	}

	wallTimeMs := time.Since(start).Milliseconds()
	stdoutPath := resolveHostPath(runSpec.StdoutPath, runSpec)
	stderrPath := resolveHostPath(runSpec.StderrPath, runSpec)
	runResult := result.RunResult{
		ExitCode:   exitCodeFromErr(waitErr, cmd.ProcessState),
		TimeMs:     cpuTimeMs(cmd.ProcessState),
		WallTimeMs: wallTimeMs,
		MemoryKB:   memoryPeakKB(cgroupPath, cmd.ProcessState),
		OutputKB:   stdoutSizeKB(stdoutPath),
		Stdout:     readLimitedFile(stdoutPath, e.cfg.StdoutStderrMaxBytes),
		Stderr:     readLimitedFile(stderrPath, e.cfg.StdoutStderrMaxBytes),
		OomKilled:  wasOomKilled(cgroupPath),
	}

	if timedOut.Load() && runResult.ExitCode == 0 {
		runResult.ExitCode = -1
	}

	if waitErr != nil && errors.Is(waitErr, context.DeadlineExceeded) {
		runResult.ExitCode = -1
	}

	if waitErr != nil && helperStderr.Len() > 0 {
		logger.Warn(ctx, "sandbox helper failed", zap.String("stderr", helperStderr.String()))
	}

	return runResult, nil
}

func exitCodeFromErr(err error, state *os.ProcessState) int {
	if state != nil {
		return state.ExitCode()
	}
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func (e *linuxEngine) KillSubmission(ctx context.Context, submissionID string) error {
	if submissionID == "" {
		return fmt.Errorf("submission id is required")
	}
	paths := e.snapshotCgroups(submissionID)
	for _, cgroupPath := range paths {
		if err := killCgroup(cgroupPath); err != nil {
			logger.Warn(ctx, "kill cgroup failed", zap.String("cgroup", cgroupPath), zap.Error(err))
		}
	}
	return nil
}

func (e *linuxEngine) registerCgroup(submissionID, cgroupPath string) {
	e.registryM.Lock()
	defer e.registryM.Unlock()
	e.registry[submissionID] = append(e.registry[submissionID], cgroupPath)
}

func (e *linuxEngine) unregisterCgroup(submissionID, cgroupPath string) {
	e.registryM.Lock()
	defer e.registryM.Unlock()
	paths := e.registry[submissionID]
	if len(paths) == 0 {
		return
	}
	updated := paths[:0]
	for _, p := range paths {
		if p != cgroupPath {
			updated = append(updated, p)
		}
	}
	if len(updated) == 0 {
		delete(e.registry, submissionID)
		return
	}
	e.registry[submissionID] = updated
}

func (e *linuxEngine) snapshotCgroups(submissionID string) []string {
	e.registryM.Lock()
	defer e.registryM.Unlock()
	paths := e.registry[submissionID]
	out := make([]string, len(paths))
	copy(out, paths)
	return out
}

func (e *linuxEngine) killProcessGroup(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

func validateRunSpec(runSpec spec.RunSpec) error {
	if runSpec.SubmissionID == "" {
		return fmt.Errorf("submission id is required")
	}
	if runSpec.TestID == "" {
		return fmt.Errorf("test id is required")
	}
	if runSpec.WorkDir == "" {
		return fmt.Errorf("work dir is required")
	}
	if len(runSpec.Cmd) == 0 {
		return fmt.Errorf("command is required")
	}
	if runSpec.Profile == "" {
		return fmt.Errorf("profile is required")
	}
	return nil
}

func jsonToPipe(req initRequest) (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	go func() {
		enc := json.NewEncoder(writer)
		err := enc.Encode(req)
		_ = writer.CloseWithError(err)
	}()
	return reader, nil
}

func buildSysProcAttr(profile security.IsolationProfile, enableNamespaces bool) *syscall.SysProcAttr {
	attr := &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
	if !enableNamespaces {
		return attr
	}

	cloneFlags := uintptr(syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC)
	if profile.DisableNetwork {
		cloneFlags |= syscall.CLONE_NEWNET
	}
	cloneFlags |= syscall.CLONE_NEWUSER

	attr.Cloneflags = cloneFlags
	attr.GidMappingsEnableSetgroups = false
	attr.UidMappings = []syscall.SysProcIDMap{{
		ContainerID: 0,
		HostID:      os.Getuid(),
		Size:        1,
	}}
	attr.GidMappings = []syscall.SysProcIDMap{{
		ContainerID: 0,
		HostID:      os.Getgid(),
		Size:        1,
	}}
	return attr
}
