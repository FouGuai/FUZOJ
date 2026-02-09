//go:build linux

package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fuzoj/internal/judge/sandbox/engine"
	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/judge/sandbox/security"
	"fuzoj/internal/judge/sandbox/spec"
)

type staticResolver struct {
	profile security.IsolationProfile
	err     error
}

func (r staticResolver) Resolve(profile string) (security.IsolationProfile, error) {
	if r.err != nil {
		return security.IsolationProfile{}, r.err
	}
	return r.profile, nil
}

func TestLinuxEngineRun(t *testing.T) {
	helperPath := buildSandboxHelper(t)
	resolver := staticResolver{profile: security.IsolationProfile{}}

	cases := []struct {
		name   string
		run    func(t *testing.T) (result.RunResult, error)
		verify func(t *testing.T, res result.RunResult, err error)
	}{
		{
			name: "cgroup_limits_applied",
			run: func(t *testing.T) (result.RunResult, error) {
				workDir := t.TempDir()
				stdoutPath := filepath.Join(workDir, "stdout.txt")
				stderrPath := filepath.Join(workDir, "stderr.txt")
				cgroupRoot := filepath.Join(workDir, "cgroup")

				cfg := engine.Config{
					CgroupRoot:       cgroupRoot,
					HelperPath:       helperPath,
					EnableSeccomp:    false,
					EnableCgroup:     true,
					EnableNamespaces: false,
				}
				eng, err := engine.NewEngine(cfg, resolver)
				if err != nil {
					t.Fatalf("create engine: %v", err)
				}

				runSpec := spec.RunSpec{
					SubmissionID: "sub-limits",
					TestID:       "t-limits",
					WorkDir:      workDir,
					Cmd:          []string{"/bin/sh", "-c", "echo ok; sleep 0.3"},
					StdoutPath:   stdoutPath,
					StderrPath:   stderrPath,
					Profile:      "default",
					Limits: spec.ResourceLimit{
						MemoryMB: 16,
						PIDs:     5,
					},
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				resultCh := make(chan result.RunResult, 1)
				errCh := make(chan error, 1)
				go func() {
					res, runErr := eng.Run(ctx, runSpec)
					resultCh <- res
					errCh <- runErr
				}()

				runDir, err := waitForRunDir(cgroupRoot, runSpec.SubmissionID, 2*time.Second)
				if err != nil {
					t.Fatalf("wait for cgroup directory: %v", err)
				}

				if data, err := os.ReadFile(filepath.Join(runDir, "pids.max")); err != nil {
					t.Fatalf("read pids.max: %v", err)
				} else if strings.TrimSpace(string(data)) != "5" {
					t.Fatalf("unexpected pids.max: %q", strings.TrimSpace(string(data)))
				}

				if data, err := os.ReadFile(filepath.Join(runDir, "memory.max")); err != nil {
					t.Fatalf("read memory.max: %v", err)
				} else if strings.TrimSpace(string(data)) != "16777216" {
					t.Fatalf("unexpected memory.max: %q", strings.TrimSpace(string(data)))
				}

				if data, err := os.ReadFile(filepath.Join(runDir, "cpu.max")); err != nil {
					t.Fatalf("read cpu.max: %v", err)
				} else if strings.TrimSpace(string(data)) != "max 100000" {
					t.Fatalf("unexpected cpu.max: %q", strings.TrimSpace(string(data)))
				}

				res := <-resultCh
				runErr := <-errCh
				return res, runErr
			},
			verify: func(t *testing.T, res result.RunResult, err error) {
				if err != nil {
					t.Fatalf("run failed: %v", err)
				}
				if res.ExitCode != 0 {
					t.Fatalf("expected exit code 0, got %d", res.ExitCode)
				}
			},
		},
		{
			name: "stats_and_resource_lifecycle",
			run: func(t *testing.T) (result.RunResult, error) {
				workDir := t.TempDir()
				stdoutPath := filepath.Join(workDir, "stdout.txt")
				stderrPath := filepath.Join(workDir, "stderr.txt")
				cgroupRoot := filepath.Join(workDir, "cgroup")

				cfg := engine.Config{
					CgroupRoot:       cgroupRoot,
					HelperPath:       helperPath,
					EnableSeccomp:    false,
					EnableCgroup:     true,
					EnableNamespaces: false,
				}
				eng, err := engine.NewEngine(cfg, resolver)
				if err != nil {
					t.Fatalf("create engine: %v", err)
				}

				runSpec := spec.RunSpec{
					SubmissionID: "sub-1",
					TestID:       "t1",
					WorkDir:      workDir,
					Cmd:          []string{"/bin/sh", "-c", "echo hello; echo oops 1>&2; sleep 0.5"},
					StdoutPath:   stdoutPath,
					StderrPath:   stderrPath,
					Profile:      "default",
					Limits: spec.ResourceLimit{
						WallTimeMs: 2000,
					},
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				resultCh := make(chan result.RunResult, 1)
				errCh := make(chan error, 1)
				go func() {
					res, runErr := eng.Run(ctx, runSpec)
					resultCh <- res
					errCh <- runErr
				}()

				runDir, err := waitForRunDir(cgroupRoot, runSpec.SubmissionID, 2*time.Second)
				if err != nil {
					t.Fatalf("wait for cgroup directory: %v", err)
				}

				if _, err := os.Stat(filepath.Join(runDir, "pids.max")); err != nil {
					t.Fatalf("expected pids.max to be initialized: %v", err)
				}

				killPath := filepath.Join(runDir, "cgroup.kill")
				if err := os.WriteFile(killPath, []byte("0"), 0600); err != nil {
					t.Fatalf("prepare cgroup.kill: %v", err)
				}

				if err := eng.KillSubmission(ctx, runSpec.SubmissionID); err != nil {
					t.Fatalf("kill submission: %v", err)
				}

				if data, err := os.ReadFile(killPath); err != nil {
					t.Fatalf("read cgroup.kill: %v", err)
				} else if strings.TrimSpace(string(data)) != "1" {
					t.Fatalf("unexpected cgroup.kill value: %q", strings.TrimSpace(string(data)))
				}

				res := <-resultCh
				runErr := <-errCh

				if _, err := os.Stat(runDir); err == nil {
					t.Fatalf("expected cgroup directory to be cleaned up")
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("stat cgroup directory: %v", err)
				}

				return res, runErr
			},
			verify: func(t *testing.T, res result.RunResult, err error) {
				if err != nil {
					t.Fatalf("run failed: %v", err)
				}
				if res.ExitCode != 0 {
					t.Fatalf("expected exit code 0, got %d", res.ExitCode)
				}
				if !strings.Contains(res.Stdout, "hello") {
					t.Fatalf("stdout missing expected content: %q", res.Stdout)
				}
				if !strings.Contains(res.Stderr, "oops") {
					t.Fatalf("stderr missing expected content: %q", res.Stderr)
				}
				if res.WallTimeMs <= 0 {
					t.Fatalf("expected wall time to be positive, got %d", res.WallTimeMs)
				}
			},
		},
		{
			name: "stdout_stderr_truncation",
			run: func(t *testing.T) (result.RunResult, error) {
				workDir := t.TempDir()
				stdoutPath := filepath.Join(workDir, "stdout.txt")
				stderrPath := filepath.Join(workDir, "stderr.txt")

				cfg := engine.Config{
					HelperPath:           helperPath,
					EnableSeccomp:        false,
					EnableCgroup:         false,
					EnableNamespaces:     false,
					StdoutStderrMaxBytes: 8,
				}
				eng, err := engine.NewEngine(cfg, resolver)
				if err != nil {
					t.Fatalf("create engine: %v", err)
				}

				runSpec := spec.RunSpec{
					SubmissionID: "sub-output",
					TestID:       "t-output",
					WorkDir:      workDir,
					Cmd:          []string{"/bin/sh", "-c", "printf '0123456789'; printf 'abcdefghij' 1>&2"},
					StdoutPath:   stdoutPath,
					StderrPath:   stderrPath,
					Profile:      "default",
					Limits: spec.ResourceLimit{
						WallTimeMs: 2000,
					},
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				return eng.Run(ctx, runSpec)
			},
			verify: func(t *testing.T, res result.RunResult, err error) {
				if err != nil {
					t.Fatalf("run failed: %v", err)
				}
				if res.ExitCode != 0 {
					t.Fatalf("expected exit code 0, got %d", res.ExitCode)
				}
				if len(res.Stdout) != 8 {
					t.Fatalf("expected stdout length 8, got %d", len(res.Stdout))
				}
				if len(res.Stderr) != 8 {
					t.Fatalf("expected stderr length 8, got %d", len(res.Stderr))
				}
			},
		},
		{
			name: "cpu_time_limit_kills_process",
			run: func(t *testing.T) (result.RunResult, error) {
				workDir := t.TempDir()
				stdoutPath := filepath.Join(workDir, "stdout.txt")
				stderrPath := filepath.Join(workDir, "stderr.txt")
				cgroupRoot := filepath.Join(workDir, "cgroup")

				cfg := engine.Config{
					CgroupRoot:       cgroupRoot,
					HelperPath:       helperPath,
					EnableSeccomp:    false,
					EnableCgroup:     true,
					EnableNamespaces: false,
				}
				eng, err := engine.NewEngine(cfg, resolver)
				if err != nil {
					t.Fatalf("create engine: %v", err)
				}

				runSpec := spec.RunSpec{
					SubmissionID: "sub-cpu",
					TestID:       "t-cpu",
					WorkDir:      workDir,
					Cmd:          []string{"/bin/sh", "-c", "sleep 2"},
					StdoutPath:   stdoutPath,
					StderrPath:   stderrPath,
					Profile:      "default",
					Limits: spec.ResourceLimit{
						CPUTimeMs:  10,
						WallTimeMs: 2000,
					},
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				resultCh := make(chan result.RunResult, 1)
				errCh := make(chan error, 1)
				go func() {
					res, runErr := eng.Run(ctx, runSpec)
					resultCh <- res
					errCh <- runErr
				}()

				runDir, err := waitForRunDir(cgroupRoot, runSpec.SubmissionID, 2*time.Second)
				if err != nil {
					t.Fatalf("wait for cgroup directory: %v", err)
				}

				cpuStat := []byte("usage_usec 20000\nuser_usec 0\nsystem_usec 0\n")
				if err := os.WriteFile(filepath.Join(runDir, "cpu.stat"), cpuStat, 0644); err != nil {
					t.Fatalf("write cpu.stat: %v", err)
				}

				res := <-resultCh
				runErr := <-errCh
				return res, runErr
			},
			verify: func(t *testing.T, res result.RunResult, err error) {
				if err != nil {
					t.Fatalf("run failed: %v", err)
				}
				if res.ExitCode != -1 {
					t.Fatalf("expected timeout exit code -1, got %d", res.ExitCode)
				}
				if res.TimeMs < 20 {
					t.Fatalf("expected cpu time >= 20, got %d", res.TimeMs)
				}
			},
		},
		{
			name: "timeout_kills_process",
			run: func(t *testing.T) (result.RunResult, error) {
				workDir := t.TempDir()
				stdoutPath := filepath.Join(workDir, "stdout.txt")
				stderrPath := filepath.Join(workDir, "stderr.txt")

				cfg := engine.Config{
					HelperPath:       helperPath,
					EnableSeccomp:    false,
					EnableCgroup:     false,
					EnableNamespaces: false,
				}
				eng, err := engine.NewEngine(cfg, resolver)
				if err != nil {
					t.Fatalf("create engine: %v", err)
				}

				runSpec := spec.RunSpec{
					SubmissionID: "sub-timeout",
					TestID:       "t-timeout",
					WorkDir:      workDir,
					Cmd:          []string{"/bin/sh", "-c", "sleep 2"},
					StdoutPath:   stdoutPath,
					StderrPath:   stderrPath,
					Profile:      "default",
					Limits: spec.ResourceLimit{
						WallTimeMs: 100,
					},
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				return eng.Run(ctx, runSpec)
			},
			verify: func(t *testing.T, res result.RunResult, err error) {
				if err != nil {
					t.Fatalf("run failed: %v", err)
				}
				if res.ExitCode != -1 {
					t.Fatalf("expected timeout exit code -1, got %d", res.ExitCode)
				}
				if res.WallTimeMs <= 0 {
					t.Fatalf("expected wall time to be positive, got %d", res.WallTimeMs)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := tc.run(t)
			tc.verify(t, res, err)
		})
	}
}

func waitForRunDir(root, submissionID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	submissionDir := filepath.Join(root, submissionID)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(submissionDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					return filepath.Join(submissionDir, entry.Name()), nil
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout waiting for cgroup directory")
}

func buildSandboxHelper(t *testing.T) string {
	t.Helper()
	helperDir := filepath.Join(t.TempDir(), "helper")
	if err := os.MkdirAll(helperDir, 0755); err != nil {
		t.Fatalf("create helper dir: %v", err)
	}

	goMod := []byte("module sandboxhelper\n\ngo 1.22\n")
	if err := os.WriteFile(filepath.Join(helperDir, "go.mod"), goMod, 0644); err != nil {
		t.Fatalf("write helper go.mod: %v", err)
	}

	if err := os.WriteFile(filepath.Join(helperDir, "main.go"), []byte(helperSource), 0644); err != nil {
		t.Fatalf("write helper main.go: %v", err)
	}

	helperPath := filepath.Join(helperDir, "sandbox-init")
	cmd := exec.Command("go", "build", "-o", helperPath, ".")
	cmd.Dir = helperDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build helper failed: %v: %s", err, string(output))
	}
	return helperPath
}

const helperSource = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

type initRequest struct {
	RunSpec runSpec ` + "`json:\"RunSpec\"`" + `
}

type runSpec struct {
	WorkDir    string   ` + "`json:\"WorkDir\"`" + `
	Cmd        []string ` + "`json:\"Cmd\"`" + `
	Env        []string ` + "`json:\"Env\"`" + `
	StdinPath  string   ` + "`json:\"StdinPath\"`" + `
	StdoutPath string   ` + "`json:\"StdoutPath\"`" + `
	StderrPath string   ` + "`json:\"StderrPath\"`" + `
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	dec := json.NewDecoder(os.Stdin)
	var req initRequest
	if err := dec.Decode(&req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if len(req.RunSpec.Cmd) == 0 {
		return fmt.Errorf("command is required")
	}
	if req.RunSpec.WorkDir == "" {
		return fmt.Errorf("work dir is required")
	}
	stdinPath := req.RunSpec.StdinPath
	if stdinPath == "" {
		stdinPath = "/dev/null"
	}
	stdoutPath := req.RunSpec.StdoutPath
	if stdoutPath == "" {
		stdoutPath = "/dev/null"
	}
	stderrPath := req.RunSpec.StderrPath
	if stderrPath == "" {
		stderrPath = "/dev/null"
	}
	stdinFile, err := os.Open(stdinPath)
	if err != nil {
		return fmt.Errorf("open stdin: %w", err)
	}
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open stdout: %w", err)
	}
	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open stderr: %w", err)
	}

	cmd := exec.Command(req.RunSpec.Cmd[0], req.RunSpec.Cmd[1:]...)
	cmd.Dir = req.RunSpec.WorkDir
	cmd.Stdin = stdinFile
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	cmd.Env = buildEnv(req.RunSpec.Env)

	err = cmd.Run()
	_ = stdinFile.Close()
	_ = stdoutFile.Close()
	_ = stderrFile.Close()
	if err != nil {
		return fmt.Errorf("run command: %w", err)
	}
	return nil
}

func buildEnv(env []string) []string {
	if len(env) > 0 {
		return env
	}
	return []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
}
`
