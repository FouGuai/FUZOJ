package tests

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"fuzoj/internal/judge/sandbox/engine"
	"fuzoj/internal/judge/sandbox/profile"
	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/judge/sandbox/runner"
	"fuzoj/internal/judge/sandbox/security"
	"fuzoj/internal/judge/sandbox/spec"
)

type fakeEngine struct {
	runResults []result.RunResult
	runErrs    []error
	runSpecs   []spec.RunSpec
}

func (f *fakeEngine) Run(ctx context.Context, runSpec spec.RunSpec) (result.RunResult, error) {
	f.runSpecs = append(f.runSpecs, runSpec)
	idx := len(f.runSpecs) - 1
	if idx < len(f.runResults) {
		if idx < len(f.runErrs) && f.runErrs[idx] != nil {
			return f.runResults[idx], f.runErrs[idx]
		}
		return f.runResults[idx], nil
	}
	return result.RunResult{}, nil
}

func (f *fakeEngine) KillSubmission(ctx context.Context, submissionID string) error {
	return nil
}

func TestCppCompileBuildsRunSpec(t *testing.T) {
	workDir := t.TempDir()
	sourcePath := filepath.Join(workDir, "src.cpp")
	if err := os.WriteFile(sourcePath, []byte("int main() {return 0;}"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	lang := profile.LanguageSpec{
		ID:               "cpp",
		SourceFile:       "main.cpp",
		BinaryFile:       "main",
		CompileEnabled:   true,
		CompileCmdTpl:    "g++ -O2 {extraFlags} -o {bin} {src}",
		RunCmdTpl:        "{bin}",
		TimeMultiplier:   2.0,
		MemoryMultiplier: 1.5,
	}
	prof := profile.TaskProfile{TaskType: profile.TaskTypeCompile}
	engine := &fakeEngine{runResults: []result.RunResult{{ExitCode: 0}}}
	r := runner.NewRunner(engine)

	req := runner.CppCompileRequest{CompileRequest: runner.CompileRequest{
		SubmissionID:      "sub-1",
		Language:          lang,
		Profile:           prof,
		WorkDir:           workDir,
		SourcePath:        sourcePath,
		ExtraCompileFlags: []string{"-pipe"},
		Limits:            spec.ResourceLimit{CPUTimeMs: 1000, MemoryMB: 256},
	}}

	if _, err := r.CompileCpp(context.Background(), req); err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	if len(engine.runSpecs) != 1 {
		t.Fatalf("expected 1 run spec, got %d", len(engine.runSpecs))
	}
	runSpec := engine.runSpecs[0]
	if runSpec.WorkDir != "/work" {
		t.Fatalf("unexpected workdir: %s", runSpec.WorkDir)
	}
	if runSpec.StderrPath != "/work/compile.log" {
		t.Fatalf("unexpected stderr path: %s", runSpec.StderrPath)
	}
	if runSpec.Limits.CPUTimeMs != 2000 {
		t.Fatalf("expected CPUTimeMs 2000, got %d", runSpec.Limits.CPUTimeMs)
	}
	if runSpec.Limits.MemoryMB != 384 {
		t.Fatalf("expected MemoryMB 384, got %d", runSpec.Limits.MemoryMB)
	}
	if len(runSpec.Cmd) == 0 || runSpec.Cmd[0] != "g++" {
		t.Fatalf("unexpected cmd: %v", runSpec.Cmd)
	}
	if len(runSpec.BindMounts) == 0 || runSpec.BindMounts[0].Source != workDir {
		t.Fatalf("expected workdir bind mount")
	}

	targetSource := filepath.Join(workDir, "main.cpp")
	if _, err := os.Stat(targetSource); err != nil {
		t.Fatalf("expected source to be copied: %v", err)
	}
}

func TestCppRunStdioRunSpec(t *testing.T) {
	workDir := t.TempDir()
	inputPath := filepath.Join(workDir, "input.src")
	answerPath := filepath.Join(workDir, "answer.src")
	if err := os.WriteFile(inputPath, []byte("1 2"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if err := os.WriteFile(answerPath, []byte("3"), 0644); err != nil {
		t.Fatalf("write answer: %v", err)
	}

	lang := profile.LanguageSpec{
		ID:         "cpp",
		SourceFile: "main.cpp",
		BinaryFile: "main",
		RunCmdTpl:  "{bin}",
	}
	prof := profile.TaskProfile{TaskType: profile.TaskTypeRun}
	engine := &fakeEngine{runResults: []result.RunResult{{ExitCode: 0}}}
	r := runner.NewRunner(engine)

	req := runner.CppRunRequest{RunRequest: runner.RunRequest{
		SubmissionID: "sub-1",
		TestID:       "t1",
		Language:     lang,
		Profile:      prof,
		WorkDir:      workDir,
		IOConfig:     runner.IOConfig{Mode: "stdio"},
		InputPath:    inputPath,
		AnswerPath:   answerPath,
		Limits:       spec.ResourceLimit{WallTimeMs: 1000},
	}}

	if _, err := r.RunCpp(context.Background(), req); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(engine.runSpecs) != 1 {
		t.Fatalf("expected 1 run spec, got %d", len(engine.runSpecs))
	}
	runSpec := engine.runSpecs[0]
	if runSpec.StdinPath != "/work/input.txt" {
		t.Fatalf("unexpected stdin path: %s", runSpec.StdinPath)
	}
	if runSpec.StdoutPath != "/work/output.txt" {
		t.Fatalf("unexpected stdout path: %s", runSpec.StdoutPath)
	}
	if runSpec.StderrPath != "/work/runtime.log" {
		t.Fatalf("unexpected stderr path: %s", runSpec.StderrPath)
	}
}

func TestCppRunVerdictMapping(t *testing.T) {
	workDir := t.TempDir()
	inputPath := filepath.Join(workDir, "input.src")
	answerPath := filepath.Join(workDir, "answer.src")
	if err := os.WriteFile(inputPath, []byte("1 2"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if err := os.WriteFile(answerPath, []byte("3"), 0644); err != nil {
		t.Fatalf("write answer: %v", err)
	}

	lang := profile.LanguageSpec{
		ID:         "cpp",
		BinaryFile: "main",
		RunCmdTpl:  "{bin}",
	}
	prof := profile.TaskProfile{TaskType: profile.TaskTypeRun}
	baseReq := runner.RunRequest{
		SubmissionID: "sub-1",
		TestID:       "t1",
		Language:     lang,
		Profile:      prof,
		WorkDir:      workDir,
		IOConfig:     runner.IOConfig{Mode: "stdio"},
		InputPath:    inputPath,
		AnswerPath:   answerPath,
		Limits:       spec.ResourceLimit{MemoryMB: 64, OutputMB: 1},
	}

	cases := []struct {
		name     string
		runRes   result.RunResult
		wantVerd result.Verdict
	}{
		{name: "tle", runRes: result.RunResult{ExitCode: -1}, wantVerd: result.VerdictTLE},
		{name: "mle", runRes: result.RunResult{ExitCode: 0, OomKilled: true}, wantVerd: result.VerdictMLE},
		{name: "ole", runRes: result.RunResult{ExitCode: 0, OutputKB: 2048}, wantVerd: result.VerdictOLE},
		{name: "re", runRes: result.RunResult{ExitCode: 2}, wantVerd: result.VerdictRE},
		{name: "ac", runRes: result.RunResult{ExitCode: 0}, wantVerd: result.VerdictAC},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := &fakeEngine{runResults: []result.RunResult{tc.runRes}}
			r := runner.NewRunner(engine)
			res, err := r.Run(context.Background(), baseReq)
			if err != nil {
				t.Fatalf("run failed: %v", err)
			}
			if res.Verdict != tc.wantVerd {
				t.Fatalf("expected verdict %s, got %s", tc.wantVerd, res.Verdict)
			}
		})
	}
}

func TestCppRunWithChecker(t *testing.T) {
	workDir := t.TempDir()
	inputPath := filepath.Join(workDir, "input.src")
	answerPath := filepath.Join(workDir, "answer.src")
	if err := os.WriteFile(inputPath, []byte("1 2"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if err := os.WriteFile(answerPath, []byte("3"), 0644); err != nil {
		t.Fatalf("write answer: %v", err)
	}

	lang := profile.LanguageSpec{
		ID:         "cpp",
		BinaryFile: "main",
		RunCmdTpl:  "{bin}",
	}
	runProf := profile.TaskProfile{TaskType: profile.TaskTypeRun}
	checkerProf := profile.TaskProfile{TaskType: profile.TaskTypeChecker}

	engine := &fakeEngine{
		runResults: []result.RunResult{
			{ExitCode: 0},
			{ExitCode: 1},
		},
	}
	r := runner.NewRunner(engine)

	req := runner.CppRunRequest{RunRequest: runner.RunRequest{
		SubmissionID: "sub-1",
		TestID:       "t1",
		Language:     lang,
		Profile:      runProf,
		WorkDir:      workDir,
		IOConfig:     runner.IOConfig{Mode: "stdio"},
		InputPath:    inputPath,
		AnswerPath:   answerPath,
		Limits:       spec.ResourceLimit{WallTimeMs: 1000},
		Checker: &runner.CheckerSpec{
			BinaryPath: "/work/checker",
		},
		CheckerProfile: &checkerProf,
	}}

	res, err := r.RunCpp(context.Background(), req)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if res.Verdict != result.VerdictWA {
		t.Fatalf("expected verdict WA, got %s", res.Verdict)
	}
	if len(engine.runSpecs) != 2 {
		t.Fatalf("expected 2 run specs, got %d", len(engine.runSpecs))
	}
	if engine.runSpecs[1].TestID != "t1-checker" {
		t.Fatalf("unexpected checker test id: %s", engine.runSpecs[1].TestID)
	}
}

func TestCppRunnerCallsEngineWithComplexProgram(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("g++ is required for this test")
	}

	helperPath := buildSandboxHelperInRepo(t)
	if err := checkHelperExecutable(helperPath); err != nil {
		t.Fatalf("sandbox helper not executable: %v", err)
	}
	workDir, err := os.MkdirTemp("", "fuzoj-runner-")
	if err != nil {
		t.Fatalf("work dir not writable: %v", err)
	}
	defer os.RemoveAll(workDir)
	resolver := staticResolver{profile: security.IsolationProfile{}}
	eng, err := engine.NewEngine(engine.Config{
		CgroupRoot:       filepath.Join(workDir, "cgroup"),
		HelperPath:       helperPath,
		EnableSeccomp:    true,
		EnableCgroup:     true,
		EnableNamespaces: true,
	}, resolver)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	r := runner.NewRunner(eng)

	sourcePath := filepath.Join(workDir, "source.cpp")
	inputPath := filepath.Join(workDir, "input.txt")

	cppSource := `#include <iostream>
#include <vector>
using namespace std;
long long prime_sum(int n){
    vector<bool> is_prime(n+1, true);
    is_prime[0]=false; is_prime[1]=false;
    for(int i=2;i*i<=n;i++){
        if(is_prime[i]){
            for(int j=i*i;j<=n;j+=i) is_prime[j]=false;
        }
    }
    long long sum=0;
    for(int i=2;i<=n;i++) if(is_prime[i]) sum+=i;
    return sum;
}
int main(){
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    if(!(cin>>n)) return 1;
    long long sum=prime_sum(n);
    long long fib=0, a=0, b=1;
    for(int i=0;i<n%40;i++){ fib=a+b; a=b; b=fib; }
    cout << sum << " " << fib << "\n";
    cerr << "n=" << n << " sum=" << sum << "\n";
    return 0;
}`
	if err := os.WriteFile(sourcePath, []byte(cppSource), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(inputPath, []byte("50000\n"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
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
		Env:            nil,
	}

	compileProfile := profile.TaskProfile{TaskType: profile.TaskTypeCompile}
	runProfile := profile.TaskProfile{TaskType: profile.TaskTypeRun}

	compileReq := runner.CppCompileRequest{CompileRequest: runner.CompileRequest{
		SubmissionID:      "sub-runner",
		Language:          lang,
		Profile:           compileProfile,
		WorkDir:           workDir,
		SourcePath:        sourcePath,
		ExtraCompileFlags: []string{},
		Limits:            spec.ResourceLimit{WallTimeMs: 5000, CPUTimeMs: 3000, MemoryMB: 512},
	}}
	compileRes, err := r.CompileCpp(context.Background(), compileReq)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("sandbox helper not executable: %v", err)
		}
		t.Fatalf("compile failed: %v", err)
	}
	if !compileRes.OK {
		logPath := filepath.Join(workDir, "compile.log")
		data, _ := os.ReadFile(logPath)
		t.Fatalf("compile not ok: %s log=%q, exit code %d, time:%d, log:%s", compileRes.Error, string(data), compileRes.ExitCode, compileRes.TimeMs, compileRes.LogPath)
	}

	runReq := runner.CppRunRequest{RunRequest: runner.RunRequest{
		SubmissionID: "sub-runner",
		TestID:       "t1",
		Language:     lang,
		Profile:      runProfile,
		WorkDir:      workDir,
		IOConfig:     runner.IOConfig{Mode: "stdio"},
		InputPath:    inputPath,
		Limits:       spec.ResourceLimit{WallTimeMs: 2000, CPUTimeMs: 1500, MemoryMB: 256, OutputMB: 64},
	}}
	start := time.Now()
	runRes, err := r.RunCpp(context.Background(), runReq)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			t.Skipf("sandbox helper not executable: %v", err)
		}
		t.Fatalf("run failed: %v", err)
	}
	t.Logf("runner stats: verdict=%s exit=%d cpu_ms=%d mem_kb=%d out_kb=%d elapsed_ms=%d stdout=%q stderr=%q",
		runRes.Verdict, runRes.ExitCode, runRes.TimeMs, runRes.MemoryKB, runRes.OutputKB, time.Since(start).Milliseconds(),
		runRes.Stdout, runRes.Stderr)
	if runRes.Verdict != result.VerdictAC {
		t.Fatalf("expected verdict AC, got %s", runRes.Verdict)
	}
	if runRes.TimeMs <= 0 {
		t.Fatalf("expected cpu time to be positive, got %d", runRes.TimeMs)
	}
}

func TestCppRunnerTimesOutInfiniteLoop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("g++ is required for this test")
	}

	helperPath := buildSandboxHelperInRepo(t)
	if err := checkHelperExecutable(helperPath); err != nil {
		t.Fatalf("sandbox helper not executable: %v", err)
	}
	workDir, err := os.MkdirTemp("", "fuzoj-runner-tle-")
	if err != nil {
		t.Fatalf("work dir not writable: %v", err)
	}
	defer os.RemoveAll(workDir)
	resolver := staticResolver{profile: security.IsolationProfile{}}
	eng, err := engine.NewEngine(engine.Config{
		CgroupRoot:       filepath.Join(workDir, "cgroup"),
		HelperPath:       helperPath,
		EnableSeccomp:    true,
		EnableCgroup:     true,
		EnableNamespaces: true,
	}, resolver)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	r := runner.NewRunner(eng)

	sourcePath := filepath.Join(workDir, "source.cpp")
	inputPath := filepath.Join(workDir, "input.txt")

	cppSource := `#include <iostream>
using namespace std;
int main(){
    volatile unsigned long long x = 0;
    while(true){ x++; }
    return 0;
}`
	if err := os.WriteFile(sourcePath, []byte(cppSource), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(inputPath, []byte(""), 0644); err != nil {
		t.Fatalf("write input: %v", err)
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
		Env:            nil,
	}

	compileProfile := profile.TaskProfile{TaskType: profile.TaskTypeCompile}
	runProfile := profile.TaskProfile{TaskType: profile.TaskTypeRun}

	compileReq := runner.CppCompileRequest{CompileRequest: runner.CompileRequest{
		SubmissionID:      "sub-runner-tle",
		Language:          lang,
		Profile:           compileProfile,
		WorkDir:           workDir,
		SourcePath:        sourcePath,
		ExtraCompileFlags: []string{},
		Limits:            spec.ResourceLimit{WallTimeMs: 50000, CPUTimeMs: 30000, MemoryMB: 256},
	}}
	compileRes, err := r.CompileCpp(context.Background(), compileReq)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("sandbox helper not executable: %v", err)
		}
		t.Fatalf("compile failed: %v", err)
	}
	if !compileRes.OK {
		logPath := filepath.Join(workDir, "compile.log")
		data, _ := os.ReadFile(logPath)
		t.Fatalf("compile not ok: %s log=%q, exit code %d, time:%d, log:%s", compileRes.Error, string(data), compileRes.ExitCode, compileRes.TimeMs, compileRes.LogPath)
	}

	runReq := runner.CppRunRequest{RunRequest: runner.RunRequest{
		SubmissionID: "sub-runner-tle",
		TestID:       "t1",
		Language:     lang,
		Profile:      runProfile,
		WorkDir:      workDir,
		IOConfig:     runner.IOConfig{Mode: "stdio"},
		InputPath:    inputPath,
		Limits:       spec.ResourceLimit{WallTimeMs: 800, CPUTimeMs: 400, MemoryMB: 128, OutputMB: 1},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	runRes, err := r.RunCpp(ctx, runReq)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			t.Skipf("sandbox helper not executable: %v", err)
		}
		t.Fatalf("run failed: %v", err)
	}
	t.Logf("runner stats: verdict=%s exit=%d cpu_ms=%d mem_kb=%d out_kb=%d elapsed_ms=%d",
		runRes.Verdict, runRes.ExitCode, runRes.TimeMs, runRes.MemoryKB, runRes.OutputKB, time.Since(start).Milliseconds())
	if runRes.Verdict != result.VerdictTLE {
		t.Fatalf("expected verdict TLE, got %s (exit=%d)", runRes.Verdict, runRes.ExitCode)
	}
}

func buildSandboxHelperInRepo(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	helperDir, err := os.MkdirTemp(wd, ".sandbox-helper-")
	if err != nil {
		t.Fatalf("create helper dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(helperDir)
	})

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

func checkHelperExecutable(path string) error {
	cmd := exec.Command(path)
	cmd.Stdin = strings.NewReader("{}")
	if err := cmd.Run(); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return err
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrPermission) {
			return err
		}
	}
	return nil
}
