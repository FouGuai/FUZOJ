package runner

import (
	"context"
	"path/filepath"

	"github.com/zeromicro/go-zero/core/logx"

	"fuzoj/services/judge_service/internal/sandbox/result"
	"fuzoj/services/judge_service/internal/sandbox/spec"
)

// CppRunner implements the compile-and-run workflow for C++ submissions.
type CppRunner struct {
	support runnerSupport
}

func newCppRunner(support runnerSupport) *CppRunner {
	return &CppRunner{support: support}
}

func (r *CppRunner) Compile(ctx context.Context, req CompileRequest) (result.CompileResult, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("compile task start submission_id=%s language_id=%s work_dir=%s", req.SubmissionID, req.Language.ID, req.WorkDir)

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

	runRes, runErr := r.support.eng.Run(ctx, runSpec)
	logContent, logErr := readCompileLog(filepath.Join(req.WorkDir, compileLogName), compileLogMaxSize)
	if logErr != nil {
		logger.Errorf("read compile log failed submission_id=%s language_id=%s err=%v", req.SubmissionID, req.Language.ID, logErr)
	}

	compileRes := result.CompileResult{
		OK:       runRes.ExitCode == 0 && runErr == nil,
		ExitCode: runRes.ExitCode,
		TimeMs:   runRes.TimeMs,
		MemoryKB: runRes.MemoryKB,
		Log:      logContent,
	}
	r.support.metrics.ObserveCompile(ctx, req.Language.ID, compileRes.OK, compileRes.TimeMs, compileRes.MemoryKB)

	if runErr != nil {
		compileRes.Error = runErr.Error()
		if compileRes.Log == "" {
			compileRes.Log = runRes.Stderr
		}
		logger.Errorf("compile run failed submission_id=%s language_id=%s err=%v", req.SubmissionID, req.Language.ID, runErr)
		return compileRes, runErr
	}
	if runRes.ExitCode != 0 {
		if compileRes.Log != "" {
			compileRes.Error = compileRes.Log
		} else {
			compileRes.Error = runRes.Stderr
			compileRes.Log = runRes.Stderr
		}
	}
	return compileRes, nil
}

func (r *CppRunner) Run(ctx context.Context, req RunRequest) (result.TestcaseResult, error) {
	limits := applyLimits(req.Limits, req.Profile.DefaultLimits, req.Language)
	return r.support.executeRun(ctx, req, limits, nil)
}
