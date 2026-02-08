//go:build linux

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	req, err := decodeRequest(os.Stdin)
	if err != nil {
		return err
	}
	if err := validateRequest(req); err != nil {
		return err
	}
	if req.EnableNs {
		if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
			return fmt.Errorf("make mount private: %w", err)
		}
	}

	if !req.EnableNs {
		if req.Isolation.RootFS != "" || len(req.RunSpec.BindMounts) > 0 {
			return fmt.Errorf("namespaces disabled with rootfs or bind mounts")
		}
	} else {
		if err := applyBindMounts(req.Isolation.RootFS, req.RunSpec.BindMounts); err != nil {
			return err
		}
		if req.Isolation.RootFS != "" {
			if err := unix.Chroot(req.Isolation.RootFS); err != nil {
				return fmt.Errorf("chroot: %w", err)
			}
			if err := os.Chdir("/"); err != nil {
				return fmt.Errorf("chdir root: %w", err)
			}
		}
	}

	if err := os.Chdir(req.RunSpec.WorkDir); err != nil {
		return fmt.Errorf("chdir workdir: %w", err)
	}

	if err := applyRlimits(req.RunSpec.Limits); err != nil {
		return err
	}

	if err := redirectIO(req.RunSpec); err != nil {
		return err
	}

	if req.EnableSeccomp && req.Isolation.SeccompProfile != "" {
		if err := applySeccomp(req.Isolation.SeccompProfile); err != nil {
			return err
		}
	}

	env := buildEnv(req.RunSpec.Env)
	os.Clearenv()
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if err := os.Setenv(parts[0], parts[1]); err != nil {
			return fmt.Errorf("set env: %w", err)
		}
	}

	cmdPath, err := exec.LookPath(req.RunSpec.Cmd[0])
	if err != nil {
		return fmt.Errorf("resolve command: %w", err)
	}
	return unix.Exec(cmdPath, req.RunSpec.Cmd, env)
}

func decodeRequest(r io.Reader) (initRequest, error) {
	dec := json.NewDecoder(r)
	var req initRequest
	if err := dec.Decode(&req); err != nil {
		return initRequest{}, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

func validateRequest(req initRequest) error {
	if len(req.RunSpec.Cmd) == 0 {
		return fmt.Errorf("command is required")
	}
	if req.RunSpec.WorkDir == "" {
		return fmt.Errorf("work dir is required")
	}
	return nil
}

func applyBindMounts(rootfs string, mounts []mountSpec) error {
	for _, m := range mounts {
		if m.Source == "" || m.Target == "" {
			return fmt.Errorf("invalid mount spec")
		}
		target := m.Target
		if rootfs != "" {
			target = filepath.Join(rootfs, m.Target)
		}
		if err := ensureMountTarget(m.Source, target); err != nil {
			return err
		}
		if err := unix.Mount(m.Source, target, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
			return fmt.Errorf("bind mount: %w", err)
		}
		if m.ReadOnly {
			if err := unix.Mount("", target, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
				return fmt.Errorf("remount readonly: %w", err)
			}
		}
	}
	if rootfs != "" {
		procPath := filepath.Join(rootfs, "proc")
		if err := os.MkdirAll(procPath, 0755); err != nil {
			return fmt.Errorf("mkdir proc: %w", err)
		}
		if err := unix.Mount("proc", procPath, "proc", 0, ""); err != nil && !errors.Is(err, unix.EBUSY) {
			return fmt.Errorf("mount proc: %w", err)
		}
	}
	return nil
}

func ensureMountTarget(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat mount source: %w", err)
	}
	if info.IsDir() {
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("mkdir mount target: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("mkdir mount target dir: %w", err)
	}
	file, err := os.OpenFile(target, os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("create mount target file: %w", err)
	}
	return file.Close()
}

func applyRlimits(limits resourceLimit) error {
	if limits.CPUTimeMs > 0 {
		seconds := uint64((limits.CPUTimeMs + 999) / 1000)
		if err := unix.Setrlimit(unix.RLIMIT_CPU, &unix.Rlimit{Cur: seconds, Max: seconds}); err != nil {
			return fmt.Errorf("set rlimit cpu: %w", err)
		}
	}
	if limits.OutputMB > 0 {
		bytes := uint64(limits.OutputMB * 1024 * 1024)
		if err := unix.Setrlimit(unix.RLIMIT_FSIZE, &unix.Rlimit{Cur: bytes, Max: bytes}); err != nil {
			return fmt.Errorf("set rlimit fsize: %w", err)
		}
	}
	if limits.StackMB > 0 {
		bytes := uint64(limits.StackMB * 1024 * 1024)
		if err := unix.Setrlimit(unix.RLIMIT_STACK, &unix.Rlimit{Cur: bytes, Max: bytes}); err != nil {
			return fmt.Errorf("set rlimit stack: %w", err)
		}
	}
	if limits.PIDs > 0 {
		val := uint64(limits.PIDs)
		if err := unix.Setrlimit(unix.RLIMIT_NPROC, &unix.Rlimit{Cur: val, Max: val}); err != nil {
			return fmt.Errorf("set rlimit nproc: %w", err)
		}
	}
	return nil
}

func redirectIO(runSpec runSpec) error {
	stdinPath := runSpec.StdinPath
	if stdinPath == "" {
		stdinPath = "/dev/null"
	}
	stdoutPath := runSpec.StdoutPath
	if stdoutPath == "" {
		stdoutPath = "/dev/null"
	}
	stderrPath := runSpec.StderrPath
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
	if err := unix.Dup2(int(stdinFile.Fd()), int(os.Stdin.Fd())); err != nil {
		return fmt.Errorf("dup stdin: %w", err)
	}
	if err := unix.Dup2(int(stdoutFile.Fd()), int(os.Stdout.Fd())); err != nil {
		return fmt.Errorf("dup stdout: %w", err)
	}
	if err := unix.Dup2(int(stderrFile.Fd()), int(os.Stderr.Fd())); err != nil {
		return fmt.Errorf("dup stderr: %w", err)
	}
	_ = stdinFile.Close()
	_ = stdoutFile.Close()
	_ = stderrFile.Close()
	return nil
}

func buildEnv(env []string) []string {
	if len(env) > 0 {
		return env
	}
	return []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
}

func applySeccomp(profilePath string) error {
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return fmt.Errorf("read seccomp profile: %w", err)
	}
	var cfg seccompConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse seccomp profile: %w", err)
	}
	defaultAction, err := parseSeccompAction(cfg.DefaultAction)
	if err != nil {
		return err
	}
	filter, err := seccomp.NewFilter(defaultAction)
	if err != nil {
		return fmt.Errorf("create seccomp filter: %w", err)
	}
	for _, rule := range cfg.Syscalls {
		action, err := parseSeccompAction(rule.Action)
		if err != nil {
			return err
		}
		for _, name := range rule.Names {
			if err := filter.AddRuleExact(name, action); err != nil {
				return fmt.Errorf("add seccomp rule: %w", err)
			}
		}
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("set no new privs: %w", err)
	}
	if err := filter.Load(); err != nil {
		return fmt.Errorf("load seccomp filter: %w", err)
	}
	return nil
}

type seccompConfig struct {
	DefaultAction string           `json:"defaultAction"`
	Syscalls      []seccompSyscall `json:"syscalls"`
}

type seccompSyscall struct {
	Names  []string `json:"names"`
	Action string   `json:"action"`
}

func parseSeccompAction(action string) (seccomp.ScmpAction, error) {
	switch strings.ToUpper(action) {
	case "SCMP_ACT_ALLOW":
		return seccomp.ActAllow, nil
	case "SCMP_ACT_KILL", "SCMP_ACT_KILL_PROCESS":
		return seccomp.ActKillProcess, nil
	default:
		return seccomp.ActKillProcess, fmt.Errorf("unsupported seccomp action: %s", action)
	}
}

type initRequest struct {
	RunSpec       runSpec          `json:"RunSpec"`
	Isolation     isolationProfile `json:"Isolation"`
	EnableSeccomp bool             `json:"EnableSeccomp"`
	EnableNs      bool             `json:"EnableNs"`
}

type runSpec struct {
	WorkDir    string        `json:"WorkDir"`
	Cmd        []string      `json:"Cmd"`
	Env        []string      `json:"Env"`
	StdinPath  string        `json:"StdinPath"`
	StdoutPath string        `json:"StdoutPath"`
	StderrPath string        `json:"StderrPath"`
	BindMounts []mountSpec   `json:"BindMounts"`
	Limits     resourceLimit `json:"Limits"`
}

type mountSpec struct {
	Source   string `json:"Source"`
	Target   string `json:"Target"`
	ReadOnly bool   `json:"ReadOnly"`
}

type resourceLimit struct {
	CPUTimeMs  int64 `json:"CPUTimeMs"`
	WallTimeMs int64 `json:"WallTimeMs"`
	MemoryMB   int64 `json:"MemoryMB"`
	StackMB    int64 `json:"StackMB"`
	OutputMB   int64 `json:"OutputMB"`
	PIDs       int64 `json:"PIDs"`
}

type isolationProfile struct {
	RootFS         string `json:"RootFS"`
	SeccompProfile string `json:"SeccompProfile"`
	DisableNetwork bool   `json:"DisableNetwork"`
}
