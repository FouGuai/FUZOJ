package sandbox_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/sandbox"
	"fuzoj/services/judge_service/internal/sandbox/engine"
	"fuzoj/services/judge_service/internal/sandbox/profile"
	"fuzoj/services/judge_service/internal/sandbox/result"
	"fuzoj/services/judge_service/internal/sandbox/runner"
	"fuzoj/services/judge_service/internal/sandbox/security"
	"fuzoj/services/judge_service/internal/sandbox/spec"
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

func TestWorkerRealRunnerScoresCpp(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("g++ is required for this test")
	}
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for this test")
	}

	helperPath := buildSandboxHelperInRepo(t)
	if err := checkHelperExecutable(helperPath); err != nil {
		t.Fatalf("sandbox helper not executable: %v", err)
	}

	workRoot := t.TempDir()
	keepRoot := t.TempDir()
	submissionID := "sub-real-runner"
	submissionPath := filepath.Join(workRoot, submissionID)
	if err := os.Symlink(keepRoot, submissionPath); err != nil {
		t.Fatalf("create submission symlink: %v", err)
	}
	sourcePath := filepath.Join(workRoot, "main.cpp")
	input1Path := filepath.Join(workRoot, "input1.txt")
	input2Path := filepath.Join(workRoot, "input2.txt")
	answer1Path := filepath.Join(workRoot, "answer1.txt")
	answer2Path := filepath.Join(workRoot, "answer2.txt")
	checkerPath := filepath.Join(workRoot, "checker.py")

	cppSource := `#include <iostream>
using namespace std;
int main() {
    long long a, b;
    if (!(cin >> a >> b)) return 1;
    cout << (a + b) << "\n";
    return 0;
}`
	if err := os.WriteFile(sourcePath, []byte(cppSource), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(input1Path, []byte("1 2\n"), 0644); err != nil {
		t.Fatalf("write input1: %v", err)
	}
	if err := os.WriteFile(input2Path, []byte("40 2\n"), 0644); err != nil {
		t.Fatalf("write input2: %v", err)
	}
	if err := os.WriteFile(answer1Path, []byte("3\n"), 0644); err != nil {
		t.Fatalf("write answer1: %v", err)
	}
	if err := os.WriteFile(answer2Path, []byte("42\n"), 0644); err != nil {
		t.Fatalf("write answer2: %v", err)
	}

	checkerSource := `import sys

def main():
    if len(sys.argv) < 4:
        print("usage: checker input output answer", file=sys.stderr)
        return 1
    _, output_path, answer_path = sys.argv[1], sys.argv[2], sys.argv[3]
    with open(output_path, "r", encoding="utf-8") as f:
        out = f.read().strip()
    with open(answer_path, "r", encoding="utf-8") as f:
        ans = f.read().strip()
    if out == ans:
        return 0
    print(f"mismatch: output={out!r} answer={ans!r}", file=sys.stderr)
    return 1

if __name__ == "__main__":
    raise SystemExit(main())
`
	if err := os.WriteFile(checkerPath, []byte(checkerSource), 0644); err != nil {
		t.Fatalf("write checker: %v", err)
	}
	for _, testID := range []string{"t1", "t2"} {
		testDir := filepath.Join(keepRoot, testID)
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatalf("create test dir: %v", err)
		}
		testCheckerPath := filepath.Join(testDir, "checker.py")
		if err := os.WriteFile(testCheckerPath, []byte(checkerSource), 0644); err != nil {
			t.Fatalf("write test checker: %v", err)
		}
	}

	resolver := staticResolver{profile: security.IsolationProfile{}}
	eng, err := engine.NewEngine(engine.Config{
		HelperPath:       helperPath,
		EnableSeccomp:    false,
		EnableCgroup:     false,
		EnableNamespaces: false,
	}, resolver)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}

	lang := profile.LanguageSpec{
		ID:             "cpp",
		Name:           "C++",
		Version:        "gnu++17",
		SourceFile:     "main.cpp",
		BinaryFile:     "main",
		CompileEnabled: true,
		CompileCmdTpl:  "g++ -std=gnu++17 -O2 -pipe -o {bin} {src}",
		RunCmdTpl:      "{bin}",
	}

	worker := sandbox.NewWorker(
		runner.NewRunner(eng),
		fakeLangRepo{spec: lang},
		fakeProfileRepo{profiles: map[profile.TaskType]profile.TaskProfile{
			profile.TaskTypeCompile: {TaskType: profile.TaskTypeCompile},
			profile.TaskTypeRun:     {TaskType: profile.TaskTypeRun},
			profile.TaskTypeChecker: {TaskType: profile.TaskTypeChecker},
		}},
	)

	req := sandbox.JudgeRequest{
		SubmissionID: submissionID,
		LanguageID:   "cpp",
		WorkRoot:     workRoot,
		SourcePath:   sourcePath,
		Tests: []sandbox.TestcaseSpec{
			{
				TestID:     "t1",
				InputPath:  input1Path,
				AnswerPath: answer1Path,
				Score:      40,
				Limits:     spec.ResourceLimit{WallTimeMs: 2000, CPUTimeMs: 1500, MemoryMB: 256},
				Checker: &sandbox.CheckerSpec{
					BinaryPath: pythonPath,
					Args:       []string{"/work/checker.py"},
				},
			},
			{
				TestID:     "t2",
				InputPath:  input2Path,
				AnswerPath: answer2Path,
				Score:      60,
				Limits:     spec.ResourceLimit{WallTimeMs: 2000, CPUTimeMs: 1500, MemoryMB: 256},
				Checker: &sandbox.CheckerSpec{
					BinaryPath: pythonPath,
					Args:       []string{"/work/checker.py"},
				},
			},
		},
	}

	res, err := worker.Execute(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			t.Skipf("sandbox helper not executable: %v", err)
		}
		t.Fatalf("execute failed: %v", err)
	}
	t.Logf("total score: %d", res.Summary.TotalScore)
	for _, tc := range res.Tests {
		if tc.TestID == "" {
			continue
		}
		logPath := filepath.Join(keepRoot, tc.TestID, "checker.log")
		data, readErr := os.ReadFile(logPath)
		if readErr != nil {
			t.Logf("checker log read failed test=%s path=%s err=%v", tc.TestID, logPath, readErr)
			continue
		}
		t.Logf("checker log test=%s path=%s content=%q", tc.TestID, logPath, strings.TrimSpace(string(data)))
	}
	if res.Verdict != result.VerdictAC {
		t.Fatalf("expected verdict AC, got %s", res.Verdict)
	}
	if res.Status != result.StatusFinished {
		t.Fatalf("expected status Finished, got %s", res.Status)
	}
	if res.Summary.TotalScore != 100 {
		t.Fatalf("expected total score 100, got %d", res.Summary.TotalScore)
	}
	if len(res.Tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(res.Tests))
	}
}
