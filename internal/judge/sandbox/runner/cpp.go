package runner

import (
	"context"

	"fuzoj/internal/judge/sandbox/result"
)

// CompileCpp compiles a C++ submission using the base compile workflow.
func (r *DefaultRunner) CompileCpp(ctx context.Context, req CppCompileRequest) (result.CompileResult, error) {
	return r.Compile(ctx, req.CompileRequest)
}

// RunCpp runs a C++ submission using the base run workflow.
func (r *DefaultRunner) RunCpp(ctx context.Context, req CppRunRequest) (result.TestcaseResult, error) {
	return r.Run(ctx, req.RunRequest)
}
