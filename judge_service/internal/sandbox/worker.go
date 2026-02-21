// Package sandbox provides the worker implementation for sandbox execution.
package sandbox

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"fuzoj/judge_service/internal/sandbox/config"
	"fuzoj/judge_service/internal/sandbox/profile"
	"fuzoj/judge_service/internal/sandbox/result"
	"fuzoj/judge_service/internal/sandbox/runner"
	"fuzoj/judge_service/internal/sandbox/spec"
	appErr "fuzoj/pkg/errors"
)

const (
	subtaskStrategyMin = "min"
)

// Worker is the sandbox scheduling unit.
// It executes compile/run workflows based on prepared local data.
type Worker struct {
	runner         runner.Runner
	langRepo       config.LanguageSpecRepository
	profileRepo    config.TaskProfileRepository
	statusReporter StatusReporter
}

// NewWorker creates a new worker with required dependencies.
func NewWorker(
	runner runner.Runner,
	langRepo config.LanguageSpecRepository,
	profileRepo config.TaskProfileRepository,
) *Worker {
	return &Worker{
		runner:      runner,
		langRepo:    langRepo,
		profileRepo: profileRepo,
	}
}

// SetStatusReporter injects a status reporter for intermediate updates.
func (w *Worker) SetStatusReporter(reporter StatusReporter) {
	w.statusReporter = reporter
}

// Execute runs a full judge workflow for one submission.
func (w *Worker) Execute(ctx context.Context, req JudgeRequest) (result.JudgeResult, error) {
	if err := validateJudgeRequest(req); err != nil {
		return result.JudgeResult{}, err
	}
	if w.runner == nil || w.langRepo == nil || w.profileRepo == nil {
		return result.JudgeResult{}, appErr.New(appErr.JudgeSystemError).WithMessage("worker dependencies are not initialized")
	}

	lang, err := w.langRepo.GetLanguageSpec(ctx, req.LanguageID)
	if err != nil {
		return result.JudgeResult{}, appErr.Wrapf(err, appErr.JudgeSystemError, "load language spec failed")
	}

	runProfile, err := w.profileRepo.GetTaskProfile(ctx, profile.TaskTypeRun, lang.ID)
	if err != nil {
		return result.JudgeResult{}, appErr.Wrapf(err, appErr.JudgeSystemError, "load run profile failed")
	}

	var compileProfile profile.TaskProfile
	if lang.CompileEnabled {
		compileProfile, err = w.profileRepo.GetTaskProfile(ctx, profile.TaskTypeCompile, lang.ID)
		if err != nil {
			return result.JudgeResult{}, appErr.Wrapf(err, appErr.JudgeSystemError, "load compile profile failed")
		}
	}

	submissionRoot := filepath.Join(req.WorkRoot, req.SubmissionID)
	resultBase := result.JudgeResult{
		SubmissionID: req.SubmissionID,
		Status:       result.StatusRunning,
		Language:     lang.ID,
	}

	totalTests := len(req.Tests)
	doneTests := 0

	if err := os.MkdirAll(submissionRoot, 0755); err != nil {
		return resultBase, appErr.Wrapf(err, appErr.JudgeSystemError, "create submission work root failed")
	}
	defer func() {
		_ = os.RemoveAll(submissionRoot)
	}()

	if lang.CompileEnabled {
		w.reportStatus(ctx, req, result.StatusCompiling, totalTests, doneTests)
		compileDir := filepath.Join(submissionRoot, "compile")
		if err := os.MkdirAll(compileDir, 0755); err != nil {
			return resultBase, appErr.Wrapf(err, appErr.JudgeSystemError, "create compile workdir failed")
		}
		compileReq := runner.CompileRequest{
			SubmissionID:      req.SubmissionID,
			Language:          lang,
			Profile:           compileProfile,
			WorkDir:           compileDir,
			SourcePath:        req.SourcePath,
			ExtraCompileFlags: req.ExtraCompileFlags,
			Limits:            spec.ResourceLimit{},
		}
		compileRes, compileErr := w.runner.Compile(ctx, compileReq)
		resultBase.Compile = &compileRes
		if compileErr != nil {
			resultBase.Status = result.StatusFailed
			resultBase.Verdict = result.VerdictSE
			return resultBase, compileErr
		}
		if !compileRes.OK {
			resultBase.Status = result.StatusFinished
			resultBase.Verdict = result.VerdictCE
			return resultBase, nil
		}
	}

	w.reportStatus(ctx, req, result.StatusRunning, totalTests, doneTests)

	testcases, subtaskIndex, err := prepareSubtasks(req)
	if err != nil {
		return resultBase, err
	}

	summary := result.SummaryStat{}
	tests := make([]result.TestcaseResult, 0, len(testcases))
	firstFailedTestID := ""
	globalFailed := false

	for _, tc := range testcases {
		if globalFailed {
			break
		}

		testWorkDir := filepath.Join(submissionRoot, tc.TestID)
		if err := os.MkdirAll(testWorkDir, 0755); err != nil {
			return resultBase, appErr.Wrapf(err, appErr.JudgeSystemError, "create test workdir failed")
		}

		if lang.CompileEnabled {
			if err := copyBinary(submissionRoot, tc.TestID, lang.BinaryFile); err != nil {
				return resultBase, err
			}
		}

		checkerSpec, checkerProfile, err := w.buildCheckerProfile(ctx, tc, req.LanguageID)
		if err != nil {
			return resultBase, err
		}

		runReq := runner.RunRequest{
			SubmissionID:   req.SubmissionID,
			TestID:         tc.TestID,
			Language:       lang,
			Profile:        runProfile,
			WorkDir:        testWorkDir,
			IOConfig:       runner.IOConfig(tc.IOConfig),
			InputPath:      tc.InputPath,
			AnswerPath:     tc.AnswerPath,
			Limits:         tc.Limits,
			Checker:        checkerSpec,
			CheckerProfile: checkerProfile,
			Score:          tc.Score,
			SubtaskID:      tc.SubtaskID,
		}

		runRes, runErr := w.runner.Run(ctx, runReq)
		if runErr != nil {
			resultBase.Status = result.StatusFailed
			resultBase.Verdict = result.VerdictSE
			return resultBase, runErr
		}

		tests = append(tests, runRes)
		doneTests++
		w.reportStatus(ctx, req, result.StatusRunning, totalTests, doneTests)
		summary.TotalTimeMs += runRes.TimeMs
		if runRes.MemoryKB > summary.MaxMemoryKB {
			summary.MaxMemoryKB = runRes.MemoryKB
		}

		updateSubtaskState(subtaskIndex, runRes)

		if runRes.Verdict != result.VerdictAC && firstFailedTestID == "" {
			firstFailedTestID = runRes.TestID
			globalFailed = true
		}
	}

	w.reportStatus(ctx, req, result.StatusJudging, totalTests, doneTests)
	summary.TotalScore = computeTotalScore(subtaskIndex, tests)
	summary.FailedTestID = firstFailedTestID

	verdict := result.VerdictAC
	if firstFailedTestID != "" {
		verdict = tests[len(tests)-1].Verdict
	}

	resultBase.Tests = tests
	resultBase.Summary = summary
	resultBase.Status = result.StatusFinished
	resultBase.Verdict = verdict

	return resultBase, nil
}

func (w *Worker) reportStatus(ctx context.Context, req JudgeRequest, status result.JudgeStatus, totalTests, doneTests int) {
	if w.statusReporter == nil {
		return
	}
	receivedAt := req.ReceivedAt
	if receivedAt == 0 {
		receivedAt = time.Now().Unix()
	}
	_ = w.statusReporter.ReportStatus(ctx, StatusUpdate{
		SubmissionID: req.SubmissionID,
		Status:       status,
		Language:     req.LanguageID,
		TotalTests:   totalTests,
		DoneTests:    doneTests,
		ReceivedAt:   receivedAt,
	})
}

type subtaskState struct {
	spec     SubtaskSpec
	expected int
	executed int
	failed   bool
}

func validateJudgeRequest(req JudgeRequest) error {
	if req.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if req.LanguageID == "" {
		return appErr.ValidationError("language_id", "required")
	}
	if req.WorkRoot == "" {
		return appErr.ValidationError("work_root", "required")
	}
	if req.SourcePath == "" {
		return appErr.ValidationError("source_path", "required")
	}
	if len(req.Tests) == 0 {
		return appErr.ValidationError("tests", "required")
	}
	for _, tc := range req.Tests {
		if tc.TestID == "" {
			return appErr.ValidationError("test_id", "required")
		}
		if tc.InputPath == "" {
			return appErr.ValidationError("input_path", "required")
		}
		if err := validateIOConfig(tc.IOConfig); err != nil {
			return err
		}
		if tc.Checker != nil && tc.AnswerPath == "" {
			return appErr.ValidationError("answer_path", "required")
		}
	}
	return nil
}

func validateIOConfig(cfg IOConfig) error {
	if cfg.Mode == "" || cfg.Mode == "stdio" {
		return nil
	}
	if cfg.Mode != "fileio" {
		return appErr.Newf(appErr.InvalidParams, "unsupported io mode: %s", cfg.Mode)
	}
	if cfg.InputFileName == "" {
		return appErr.ValidationError("input_file_name", "required")
	}
	if cfg.OutputFileName == "" {
		return appErr.ValidationError("output_file_name", "required")
	}
	return nil
}

func prepareSubtasks(req JudgeRequest) ([]TestcaseSpec, map[string]*subtaskState, error) {
	subtaskIndex := make(map[string]*subtaskState)
	for _, st := range req.Subtasks {
		strategy := st.Strategy
		if strategy == "" {
			strategy = subtaskStrategyMin
		}
		if strategy != subtaskStrategyMin {
			return nil, nil, appErr.Newf(appErr.InvalidParams, "unsupported subtask strategy: %s", strategy)
		}
		st.Strategy = strategy
		subtaskIndex[st.ID] = &subtaskState{spec: st}
	}

	for _, tc := range req.Tests {
		if tc.SubtaskID == "" {
			continue
		}
		state, ok := subtaskIndex[tc.SubtaskID]
		if !ok {
			return nil, nil, appErr.ValidationError("subtask_id", "not_found")
		}
		state.expected++
	}
	return req.Tests, subtaskIndex, nil
}

func updateSubtaskState(subtaskIndex map[string]*subtaskState, res result.TestcaseResult) {
	if res.SubtaskID == "" {
		return
	}
	state, ok := subtaskIndex[res.SubtaskID]
	if !ok {
		return
	}
	state.executed++
	if res.Verdict != result.VerdictAC {
		state.failed = true
	}
}

func computeTotalScore(subtaskIndex map[string]*subtaskState, tests []result.TestcaseResult) int {
	if len(subtaskIndex) == 0 {
		total := 0
		for _, tc := range tests {
			if tc.Verdict == result.VerdictAC {
				total += tc.Score
			}
		}
		return total
	}

	total := 0
	for _, state := range subtaskIndex {
		if state.expected == 0 {
			continue
		}
		if state.executed < state.expected {
			continue
		}
		if state.failed {
			continue
		}
		total += state.spec.Score
	}
	return total
}

func (w *Worker) buildCheckerProfile(ctx context.Context, tc TestcaseSpec, defaultLanguageID string) (*runner.CheckerSpec, *profile.TaskProfile, error) {
	if tc.Checker == nil {
		return nil, nil, nil
	}
	checkerLang := tc.CheckerLanguageID
	if checkerLang == "" {
		checkerLang = defaultLanguageID
	}
	checkerProfile, err := w.profileRepo.GetTaskProfile(ctx, profile.TaskTypeChecker, checkerLang)
	if err != nil {
		return nil, nil, appErr.Wrapf(err, appErr.JudgeSystemError, "load checker profile failed")
	}
	return &runner.CheckerSpec{
		BinaryPath: tc.Checker.BinaryPath,
		Args:       tc.Checker.Args,
		Env:        tc.Checker.Env,
		Limits:     tc.Checker.Limits,
	}, &checkerProfile, nil
}

func copyBinary(submissionRoot, testID, binaryName string) error {
	if binaryName == "" {
		return appErr.New(appErr.InvalidParams).WithMessage("binary file name is required")
	}
	src := filepath.Join(submissionRoot, "compile", binaryName)
	dstDir := filepath.Join(submissionRoot, testID)
	dst := filepath.Join(dstDir, binaryName)

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "create test workdir failed")
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "open compiled binary failed")
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "create test binary failed")
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "copy compiled binary failed")
	}
	if err := dstFile.Chmod(0755); err != nil {
		return appErr.Wrapf(err, appErr.JudgeSystemError, "chmod test binary failed")
	}
	return nil
}
