package runner

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/shlex"

	"fuzoj/judge_service/internal/sandbox/engine"
	"fuzoj/judge_service/internal/sandbox/observer"
	"fuzoj/judge_service/internal/sandbox/profile"
	"fuzoj/judge_service/internal/sandbox/result"
	"fuzoj/judge_service/internal/sandbox/spec"
	appErr "fuzoj/pkg/errors"
)

const (
	containerWorkDir  = "/work"
	defaultInputName  = "input.txt"
	defaultOutputName = "output.txt"
	defaultAnswerName = "answer.txt"
	compileLogName    = "compile.log"
	runtimeLogName    = "runtime.log"
	checkerLogName    = "checker.log"
)

// DefaultRunner implements compile/run workflows for supported languages.
type DefaultRunner struct {
	eng     engine.Engine
	metrics observer.MetricsRecorder
}

// NewRunner creates a new runner backed by the sandbox engine.
func NewRunner(eng engine.Engine) *DefaultRunner {
	return NewRunnerWithObserver(eng, observer.NoopMetricsRecorder{})
}

// NewRunnerWithObserver creates a new runner with metrics hooks.
func NewRunnerWithObserver(eng engine.Engine, metrics observer.MetricsRecorder) *DefaultRunner {
	if metrics == nil {
		metrics = observer.NoopMetricsRecorder{}
	}
	return &DefaultRunner{eng: eng, metrics: metrics}
}

func (r *DefaultRunner) Compile(ctx context.Context, req CompileRequest) (result.CompileResult, error) {
	if err := validateCompileRequest(req); err != nil {
		return result.CompileResult{}, err
	}
	if !req.Language.CompileEnabled {
		return result.CompileResult{OK: true}, nil
	}
	if err := prepareWorkDir(req.WorkDir); err != nil {
		return result.CompileResult{}, err
	}
	if err := writeSourceFile(req.WorkDir, req.SourcePath, req.Language.SourceFile); err != nil {
		return result.CompileResult{}, err
	}

	limits := applyLimits(req.Limits, req.Profile.DefaultLimits, req.Language)
	cmd, err := buildCommand(req.Language.CompileCmdTpl, req.Language, req.ExtraCompileFlags)
	if err != nil {
		return result.CompileResult{}, err
	}

	runSpec := spec.RunSpec{
		SubmissionID: req.SubmissionID,
		TestID:       "compile",
		WorkDir:      containerWorkDir,
		Cmd:          cmd,
		Env:          req.Language.Env,
		StdoutPath:   "",
		StderrPath:   filepath.Join(containerWorkDir, compileLogName),
		Profile:      profileName(req.Language.ID, req.Profile.TaskType),
		Limits:       limits,
		BindMounts: []spec.MountSpec{{
			Source:   req.WorkDir,
			Target:   containerWorkDir,
			ReadOnly: false,
		}},
	}

	runRes, err := r.eng.Run(ctx, runSpec)
	logPath := filepath.Join(req.WorkDir, compileLogName)
	compileRes := result.CompileResult{
		OK:       runRes.ExitCode == 0 && err == nil,
		ExitCode: runRes.ExitCode,
		TimeMs:   runRes.TimeMs,
		MemoryKB: runRes.MemoryKB,
		LogPath:  logPath,
	}
	r.metrics.ObserveCompile(ctx, req.Language.ID, compileRes.OK, compileRes.TimeMs, compileRes.MemoryKB)
	if err != nil {
		compileRes.Error = err.Error()
		return compileRes, err
	}
	if runRes.ExitCode != 0 {
		compileRes.Error = runRes.Stderr
	}
	return compileRes, nil
}

func (r *DefaultRunner) Run(ctx context.Context, req RunRequest) (result.TestcaseResult, error) {
	if req.IOConfig.Mode == "" {
		req.IOConfig.Mode = "stdio"
	}
	if err := validateRunRequest(req); err != nil {
		return result.TestcaseResult{}, err
	}
	if err := prepareWorkDir(req.WorkDir); err != nil {
		return result.TestcaseResult{}, err
	}

	limits := applyLimits(req.Limits, req.Profile.DefaultLimits, req.Language)
	runSpec, runtimeLogPath, outputName, err := buildRunSpec(req, limits)
	if err != nil {
		return result.TestcaseResult{}, err
	}

	runRes, runErr := r.eng.Run(ctx, runSpec)
	if runErr != nil {
		r.metrics.ObserveRun(ctx, req.Language.ID, string(result.VerdictSE), runRes.TimeMs, runRes.MemoryKB, runRes.OutputKB)
		return result.TestcaseResult{
			TestID:         req.TestID,
			Verdict:        result.VerdictSE,
			RuntimeLogPath: runtimeLogPath,
		}, runErr
	}

	verdict := mapRunVerdict(runRes, limits)
	checkerLogPath := ""
	if verdict == result.VerdictAC && req.Checker != nil && req.CheckerProfile != nil {
		checkerRes, checkerErr := r.runChecker(ctx, req, outputName)
		checkerLogPath = checkerRes.LogPath
		if checkerErr != nil {
			return result.TestcaseResult{
				TestID:         req.TestID,
				Verdict:        result.VerdictSE,
				RuntimeLogPath: runtimeLogPath,
				CheckerLogPath: checkerLogPath,
			}, checkerErr
		}
		if checkerRes.ExitCode != 0 {
			verdict = result.VerdictWA
		}
	}

	res := result.TestcaseResult{
		TestID:         req.TestID,
		Verdict:        verdict,
		TimeMs:         runRes.TimeMs,
		MemoryKB:       runRes.MemoryKB,
		OutputKB:       runRes.OutputKB,
		ExitCode:       runRes.ExitCode,
		RuntimeLogPath: runtimeLogPath,
		CheckerLogPath: checkerLogPath,
		Stdout:         runRes.Stdout,
		Stderr:         runRes.Stderr,
		Score:          req.Score,
		SubtaskID:      req.SubtaskID,
	}
	r.metrics.ObserveRun(ctx, req.Language.ID, string(verdict), res.TimeMs, res.MemoryKB, res.OutputKB)
	return res, nil
}

func (r *DefaultRunner) runChecker(ctx context.Context, req RunRequest, outputName string) (checkerRunResult, error) {
	if req.Checker == nil || req.CheckerProfile == nil {
		return checkerRunResult{}, appErr.ValidationError("checker", "required")
	}
	if req.AnswerPath == "" {
		return checkerRunResult{}, appErr.ValidationError("answer_path", "required")
	}
	if req.InputPath == "" {
		return checkerRunResult{}, appErr.ValidationError("input_path", "required")
	}

	checkerLimits := applyLimits(req.Checker.Limits, req.CheckerProfile.DefaultLimits, req.Language)
	cmd := append([]string{req.Checker.BinaryPath}, req.Checker.Args...)
	cmd = append(cmd,
		filepath.Join(containerWorkDir, inputName(req.IOConfig)),
		filepath.Join(containerWorkDir, outputName),
		filepath.Join(containerWorkDir, defaultAnswerName),
	)

	runSpec := spec.RunSpec{
		SubmissionID: req.SubmissionID,
		TestID:       req.TestID + "-checker",
		WorkDir:      containerWorkDir,
		Cmd:          cmd,
		Env:          req.Checker.Env,
		StdoutPath:   "",
		StderrPath:   filepath.Join(containerWorkDir, checkerLogName),
		Profile:      profileName(req.Language.ID, req.CheckerProfile.TaskType),
		Limits:       checkerLimits,
		BindMounts: buildBindMounts(req.WorkDir, []spec.MountSpec{
			{Source: req.InputPath, Target: filepath.Join(containerWorkDir, inputName(req.IOConfig)), ReadOnly: true},
			{Source: req.AnswerPath, Target: filepath.Join(containerWorkDir, defaultAnswerName), ReadOnly: true},
			{Source: filepath.Join(req.WorkDir, outputName), Target: filepath.Join(containerWorkDir, outputName), ReadOnly: true},
		}),
	}

	runRes, err := r.eng.Run(ctx, runSpec)
	return checkerRunResult{
		ExitCode: runRes.ExitCode,
		LogPath:  filepath.Join(req.WorkDir, checkerLogName),
	}, err
}

type checkerRunResult struct {
	ExitCode int
	LogPath  string
}

func validateCompileRequest(req CompileRequest) error {
	if req.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if req.WorkDir == "" {
		return appErr.ValidationError("work_dir", "required")
	}
	if req.SourcePath == "" {
		return appErr.ValidationError("source_path", "required")
	}
	if req.Language.ID == "" {
		return appErr.ValidationError("language_id", "required")
	}
	if req.Profile.TaskType == "" {
		return appErr.ValidationError("task_profile", "required")
	}
	return nil
}

func validateRunRequest(req RunRequest) error {
	if req.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if req.TestID == "" {
		return appErr.ValidationError("test_id", "required")
	}
	if req.WorkDir == "" {
		return appErr.ValidationError("work_dir", "required")
	}
	if req.Language.ID == "" {
		return appErr.ValidationError("language_id", "required")
	}
	if req.Profile.TaskType == "" {
		return appErr.ValidationError("task_profile", "required")
	}
	switch req.IOConfig.Mode {
	case "stdio", "fileio":
	default:
		return appErr.Newf(appErr.InvalidParams, "unsupported io mode: %s", req.IOConfig.Mode)
	}
	if req.InputPath == "" {
		return appErr.ValidationError("input_path", "required")
	}
	if req.IOConfig.Mode == "fileio" {
		if req.IOConfig.InputFileName == "" {
			return appErr.ValidationError("input_file_name", "required")
		}
		if req.IOConfig.OutputFileName == "" {
			return appErr.ValidationError("output_file_name", "required")
		}
	}
	return nil
}

func buildRunSpec(req RunRequest, limits spec.ResourceLimit) (spec.RunSpec, string, string, error) {
	cmd, err := buildCommand(req.Language.RunCmdTpl, req.Language, nil)
	if err != nil {
		return spec.RunSpec{}, "", "", err
	}

	input := inputName(req.IOConfig)
	output := outputName(req.IOConfig)
	stderrPath := filepath.Join(containerWorkDir, runtimeLogName)
	stdinPath := ""
	stdoutPath := ""
	if req.IOConfig.Mode == "" || req.IOConfig.Mode == "stdio" {
		stdinPath = filepath.Join(containerWorkDir, input)
		stdoutPath = filepath.Join(containerWorkDir, output)
	}

	runSpec := spec.RunSpec{
		SubmissionID: req.SubmissionID,
		TestID:       req.TestID,
		WorkDir:      containerWorkDir,
		Cmd:          cmd,
		Env:          req.Language.Env,
		StdinPath:    stdinPath,
		StdoutPath:   stdoutPath,
		StderrPath:   stderrPath,
		Profile:      profileName(req.Language.ID, req.Profile.TaskType),
		Limits:       limits,
		BindMounts: buildBindMounts(req.WorkDir, []spec.MountSpec{
			{Source: req.InputPath, Target: filepath.Join(containerWorkDir, input), ReadOnly: true},
			{Source: req.AnswerPath, Target: filepath.Join(containerWorkDir, defaultAnswerName), ReadOnly: true},
		}),
	}

	runtimeLogPath := filepath.Join(req.WorkDir, runtimeLogName)
	return runSpec, runtimeLogPath, output, nil
}

func buildBindMounts(workDir string, extra []spec.MountSpec) []spec.MountSpec {
	mounts := []spec.MountSpec{{
		Source:   workDir,
		Target:   containerWorkDir,
		ReadOnly: false,
	}}
	for _, m := range extra {
		if m.Source == "" || m.Target == "" {
			continue
		}
		mounts = append(mounts, m)
	}
	return mounts
}

func inputName(cfg IOConfig) string {
	if cfg.Mode == "fileio" && cfg.InputFileName != "" {
		return cfg.InputFileName
	}
	return defaultInputName
}

func outputName(cfg IOConfig) string {
	if cfg.Mode == "fileio" && cfg.OutputFileName != "" {
		return cfg.OutputFileName
	}
	return defaultOutputName
}

func buildCommand(tpl string, lang profile.LanguageSpec, extraFlags []string) ([]string, error) {
	if strings.TrimSpace(tpl) == "" {
		return nil, appErr.New(appErr.InvalidParams).WithMessage("command template is required")
	}
	expanded := tpl
	expanded = strings.ReplaceAll(expanded, "{src}", filepath.Join(containerWorkDir, lang.SourceFile))
	expanded = strings.ReplaceAll(expanded, "{bin}", filepath.Join(containerWorkDir, lang.BinaryFile))
	if strings.Contains(expanded, "{extraFlags}") {
		expanded = strings.ReplaceAll(expanded, "{extraFlags}", strings.Join(extraFlags, " "))
	}
	fields, err := shlex.Split(expanded)
	if err != nil {
		return nil, appErr.Wrapf(err, appErr.InvalidParams, "parse command template failed")
	}
	if len(fields) == 0 {
		return nil, appErr.New(appErr.InvalidParams).WithMessage("command is empty after expansion")
	}
	return fields, nil
}

func applyLimits(override, defaults spec.ResourceLimit, lang profile.LanguageSpec) spec.ResourceLimit {
	merged := mergeLimits(defaults, override)
	return applyMultipliers(merged, lang)
}

func mergeLimits(base, override spec.ResourceLimit) spec.ResourceLimit {
	if override.CPUTimeMs > 0 {
		base.CPUTimeMs = override.CPUTimeMs
	}
	if override.WallTimeMs > 0 {
		base.WallTimeMs = override.WallTimeMs
	}
	if override.MemoryMB > 0 {
		base.MemoryMB = override.MemoryMB
	}
	if override.StackMB > 0 {
		base.StackMB = override.StackMB
	}
	if override.OutputMB > 0 {
		base.OutputMB = override.OutputMB
	}
	if override.PIDs > 0 {
		base.PIDs = override.PIDs
	}
	return base
}

func applyMultipliers(limits spec.ResourceLimit, lang profile.LanguageSpec) spec.ResourceLimit {
	limits.CPUTimeMs = scaleLimit(limits.CPUTimeMs, lang.TimeMultiplier)
	limits.WallTimeMs = scaleLimit(limits.WallTimeMs, lang.TimeMultiplier)
	limits.MemoryMB = scaleLimit(limits.MemoryMB, lang.MemoryMultiplier)
	return limits
}

func scaleLimit(value int64, multiplier float64) int64 {
	if value <= 0 {
		return 0
	}
	if multiplier <= 0 {
		return value
	}
	return int64(math.Ceil(float64(value) * multiplier))
}

func mapRunVerdict(res result.RunResult, limits spec.ResourceLimit) result.Verdict {
	if res.ExitCode == -1 {
		return result.VerdictTLE
	}
	if res.OomKilled {
		return result.VerdictMLE
	}
	if limits.MemoryMB > 0 && res.MemoryKB > limits.MemoryMB*1024 {
		return result.VerdictMLE
	}
	if limits.OutputMB > 0 && res.OutputKB > limits.OutputMB*1024 {
		return result.VerdictOLE
	}
	if res.ExitCode != 0 {
		return result.VerdictRE
	}
	return result.VerdictAC
}

func profileName(languageID string, taskType profile.TaskType) string {
	if languageID == "" {
		return string(taskType)
	}
	return fmt.Sprintf("%s-%s", languageID, taskType)
}

func prepareWorkDir(workDir string) error {
	if workDir == "" {
		return appErr.ValidationError("work_dir", "required")
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return appErr.Wrapf(err, appErr.InternalServerError, "create work dir failed")
	}
	return nil
}

func writeSourceFile(workDir, sourcePath, targetName string) error {
	if targetName == "" {
		return appErr.ValidationError("source_file_name", "required")
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return appErr.Wrapf(err, appErr.InternalServerError, "read source failed")
	}
	targetPath := filepath.Join(workDir, targetName)
	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return appErr.Wrapf(err, appErr.InternalServerError, "write source failed")
	}
	return nil
}
