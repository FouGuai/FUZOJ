package runner

import (
	"context"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/sandbox/engine"
	"fuzoj/services/judge_service/internal/sandbox/observer"
	"fuzoj/services/judge_service/internal/sandbox/result"
)

// LanguageDispatchRunner routes requests to the language-specific runner.
type LanguageDispatchRunner struct {
	cpp Runner
	py  Runner
}

// NewRunner creates a dispatch runner backed by language-specific implementations.
func NewRunner(eng engine.Engine) Runner {
	return NewRunnerWithObserver(eng, observer.NoopMetricsRecorder{})
}

// NewRunnerWithObserver creates a dispatch runner with metrics hooks.
func NewRunnerWithObserver(eng engine.Engine, metrics observer.MetricsRecorder) Runner {
	support := newRunnerSupport(eng, metrics)
	return &LanguageDispatchRunner{
		cpp: newCppRunner(support),
		py:  newPythonRunner(support),
	}
}

func (r *LanguageDispatchRunner) Compile(ctx context.Context, req CompileRequest) (result.CompileResult, error) {
	selected, err := r.pick(req.Language.ID)
	if err != nil {
		return result.CompileResult{}, err
	}
	return selected.Compile(ctx, req)
}

func (r *LanguageDispatchRunner) Run(ctx context.Context, req RunRequest) (result.TestcaseResult, error) {
	selected, err := r.pick(req.Language.ID)
	if err != nil {
		return result.TestcaseResult{}, err
	}
	return selected.Run(ctx, req)
}

func (r *LanguageDispatchRunner) pick(languageID string) (Runner, error) {
	if languageID == "" {
		return nil, appErr.ValidationError("language_id", "required")
	}
	switch languageID {
	case "cpp":
		return r.cpp, nil
	case "py":
		return r.py, nil
	default:
		return nil, appErr.New(appErr.LanguageNotSupported).WithMessage("language not supported")
	}
}
