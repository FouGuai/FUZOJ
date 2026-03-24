package runner

import (
	"context"

	"fuzoj/services/judge_service/internal/sandbox/result"
)

// PythonRunner implements the script-run workflow for Python submissions.
type PythonRunner struct {
	support runnerSupport
}

func newPythonRunner(support runnerSupport) *PythonRunner {
	return &PythonRunner{support: support}
}

func (r *PythonRunner) Compile(ctx context.Context, req CompileRequest) (result.CompileResult, error) {
	if err := validateCompileRequest(req); err != nil {
		return result.CompileResult{}, err
	}
	return result.CompileResult{OK: true}, nil
}

func (r *PythonRunner) Run(ctx context.Context, req RunRequest) (result.TestcaseResult, error) {
	limits := applyLimits(req.Limits, req.Profile.DefaultLimits, req.Language)
	return r.support.executeRun(ctx, req, limits, func() error {
		return writeSourceFile(req.WorkDir, req.SourcePath, req.Language.SourceFile)
	})
}
