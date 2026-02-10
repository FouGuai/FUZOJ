package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fuzoj/internal/judge/sandbox"
	"fuzoj/internal/judge/sandbox/profile"
	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/judge/sandbox/runner"
	"fuzoj/internal/judge/sandbox/spec"
	pkgerrors "fuzoj/pkg/errors"
)

type fakeRunner struct {
	compileRes result.CompileResult
	compileErr error
	runResults []result.TestcaseResult
	runErrs    []error
	runReqs    []runner.RunRequest
}

func (f *fakeRunner) Compile(ctx context.Context, req runner.CompileRequest) (result.CompileResult, error) {
	return f.compileRes, f.compileErr
}

func (f *fakeRunner) Run(ctx context.Context, req runner.RunRequest) (result.TestcaseResult, error) {
	f.runReqs = append(f.runReqs, req)
	idx := len(f.runReqs) - 1
	if idx < len(f.runResults) {
		if idx < len(f.runErrs) && f.runErrs[idx] != nil {
			return f.runResults[idx], f.runErrs[idx]
		}
		return f.runResults[idx], nil
	}
	return result.TestcaseResult{TestID: req.TestID, Verdict: result.VerdictAC}, nil
}

type fakeLangRepo struct {
	spec profile.LanguageSpec
	err  error
}

func (f fakeLangRepo) GetLanguageSpec(ctx context.Context, id string) (profile.LanguageSpec, error) {
	return f.spec, f.err
}

type fakeProfileRepo struct {
	profiles map[profile.TaskType]profile.TaskProfile
	err      error
}

func (f fakeProfileRepo) GetTaskProfile(ctx context.Context, taskType profile.TaskType, languageID string) (profile.TaskProfile, error) {
	if f.err != nil {
		return profile.TaskProfile{}, f.err
	}
	if prof, ok := f.profiles[taskType]; ok {
		return prof, nil
	}
	return profile.TaskProfile{}, pkgerrors.New(pkgerrors.NotFound)
}

func TestWorkerCompileFail(t *testing.T) {
	workRoot := t.TempDir()
	sourcePath := filepath.Join(workRoot, "main.cpp")
	if err := os.WriteFile(sourcePath, []byte("int main(){return 0;}"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	inputPath := filepath.Join(workRoot, "input.txt")
	if err := os.WriteFile(inputPath, []byte("1\n"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	lang := profile.LanguageSpec{
		ID:             "cpp",
		SourceFile:     "main.cpp",
		BinaryFile:     "main",
		CompileEnabled: true,
	}

	r := &fakeRunner{compileRes: result.CompileResult{OK: false, ExitCode: 1}}
	worker := sandbox.NewWorker(r, fakeLangRepo{spec: lang}, fakeProfileRepo{
		profiles: map[profile.TaskType]profile.TaskProfile{
			profile.TaskTypeCompile: {TaskType: profile.TaskTypeCompile},
			profile.TaskTypeRun:     {TaskType: profile.TaskTypeRun},
		},
	})

	req := sandbox.JudgeRequest{
		SubmissionID: "sub-1",
		LanguageID:   "cpp",
		WorkRoot:     workRoot,
		SourcePath:   sourcePath,
		Tests: []sandbox.TestcaseSpec{{
			TestID:    "t1",
			InputPath: inputPath,
			Limits:    spec.ResourceLimit{CPUTimeMs: 100},
		}},
	}

	res, err := worker.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("expected compile failure to return nil error, got %v", err)
	}
	if res.Verdict != result.VerdictCE {
		t.Fatalf("expected verdict CE, got %s", res.Verdict)
	}
	if res.Status != result.StatusFinished {
		t.Fatalf("expected status Finished, got %s", res.Status)
	}
	if len(res.Tests) != 0 {
		t.Fatalf("expected no tests executed, got %d", len(res.Tests))
	}
}

func TestWorkerEarlyStopOnNonAC(t *testing.T) {
	workRoot := t.TempDir()
	sourcePath := filepath.Join(workRoot, "main.cpp")
	if err := os.WriteFile(sourcePath, []byte("int main(){return 0;}"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	inputPath := filepath.Join(workRoot, "input.txt")
	if err := os.WriteFile(inputPath, []byte("1\n"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	lang := profile.LanguageSpec{
		ID:             "py",
		SourceFile:     "main.py",
		BinaryFile:     "",
		CompileEnabled: false,
	}

	r := &fakeRunner{
		runResults: []result.TestcaseResult{
			{TestID: "t1", Verdict: result.VerdictTLE},
			{TestID: "t2", Verdict: result.VerdictAC},
		},
	}
	worker := sandbox.NewWorker(r, fakeLangRepo{spec: lang}, fakeProfileRepo{
		profiles: map[profile.TaskType]profile.TaskProfile{
			profile.TaskTypeRun: {TaskType: profile.TaskTypeRun},
		},
	})

	req := sandbox.JudgeRequest{
		SubmissionID: "sub-2",
		LanguageID:   "py",
		WorkRoot:     workRoot,
		SourcePath:   sourcePath,
		Tests: []sandbox.TestcaseSpec{
			{TestID: "t1", InputPath: inputPath, Score: 10},
			{TestID: "t2", InputPath: inputPath, Score: 10},
		},
	}

	res, err := worker.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(r.runReqs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(r.runReqs))
	}
	if res.Verdict != result.VerdictTLE {
		t.Fatalf("expected verdict TLE, got %s", res.Verdict)
	}
	if res.Summary.FailedTestID != "t1" {
		t.Fatalf("expected failed test t1, got %s", res.Summary.FailedTestID)
	}
}

func TestWorkerSubtaskMinScore(t *testing.T) {
	workRoot := t.TempDir()
	sourcePath := filepath.Join(workRoot, "main.cpp")
	if err := os.WriteFile(sourcePath, []byte("int main(){return 0;}"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	inputPath := filepath.Join(workRoot, "input.txt")
	if err := os.WriteFile(inputPath, []byte("1\n"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	lang := profile.LanguageSpec{
		ID:             "py",
		SourceFile:     "main.py",
		BinaryFile:     "",
		CompileEnabled: false,
	}
	r := &fakeRunner{
		runResults: []result.TestcaseResult{
			{TestID: "t1", Verdict: result.VerdictAC, Score: 10, SubtaskID: "s1"},
			{TestID: "t2", Verdict: result.VerdictAC, Score: 10, SubtaskID: "s1"},
		},
	}
	worker := sandbox.NewWorker(r, fakeLangRepo{spec: lang}, fakeProfileRepo{
		profiles: map[profile.TaskType]profile.TaskProfile{
			profile.TaskTypeRun: {TaskType: profile.TaskTypeRun},
		},
	})

	req := sandbox.JudgeRequest{
		SubmissionID: "sub-3",
		LanguageID:   "py",
		WorkRoot:     workRoot,
		SourcePath:   sourcePath,
		Tests: []sandbox.TestcaseSpec{
			{TestID: "t1", InputPath: inputPath, Score: 10, SubtaskID: "s1"},
			{TestID: "t2", InputPath: inputPath, Score: 10, SubtaskID: "s1"},
		},
		Subtasks: []sandbox.SubtaskSpec{{
			ID:       "s1",
			Score:    100,
			Strategy: "min",
		}},
	}

	res, err := worker.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if res.Summary.TotalScore != 100 {
		t.Fatalf("expected total score 100, got %d", res.Summary.TotalScore)
	}
}

func TestWorkerInvalidRequest(t *testing.T) {
	worker := sandbox.NewWorker(&fakeRunner{}, fakeLangRepo{}, fakeProfileRepo{})
	_, err := worker.Execute(context.Background(), sandbox.JudgeRequest{})
	if err == nil {
		t.Fatalf("expected error for invalid request")
	}
	if got := pkgerrors.GetCode(err); got != pkgerrors.ValidationFailed {
		t.Fatalf("expected ValidationFailed, got %v", got)
	}
}
