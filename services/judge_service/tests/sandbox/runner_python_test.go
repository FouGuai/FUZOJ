package sandbox_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fuzoj/services/judge_service/internal/sandbox/profile"
	"fuzoj/services/judge_service/internal/sandbox/runner"
	"fuzoj/services/judge_service/internal/sandbox/spec"
)

func TestPythonCompileReturnsOK(t *testing.T) {
	workDir := t.TempDir()
	sourcePath := filepath.Join(workDir, "src.py")
	if err := os.WriteFile(sourcePath, []byte("print('ok')\n"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	r := runner.NewRunner(&fakeEngine{})
	res, err := r.Compile(context.Background(), runner.CompileRequest{
		SubmissionID: "sub-py-compile",
		Language: profile.LanguageSpec{
			ID:             "py",
			SourceFile:     "main.py",
			CompileEnabled: false,
			RunCmdTpl:      "python3 {src}",
		},
		Profile:    profile.TaskProfile{TaskType: profile.TaskTypeCompile},
		WorkDir:    workDir,
		SourcePath: sourcePath,
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected python compile to be a no-op success")
	}
}

func TestPythonRunWritesSourceAndUsesInterpreter(t *testing.T) {
	workDir := t.TempDir()
	inputPath := filepath.Join(workDir, "input.txt")
	answerPath := filepath.Join(workDir, "answer.txt")
	sourcePath := filepath.Join(workDir, "source.py")
	if err := os.WriteFile(inputPath, []byte("1\n"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if err := os.WriteFile(answerPath, []byte("1\n"), 0644); err != nil {
		t.Fatalf("write answer: %v", err)
	}
	sourceContent := "print('hello')\n"
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	engine := &fakeEngine{}
	r := runner.NewRunner(engine)
	_, err := r.Run(context.Background(), runner.RunRequest{
		SubmissionID: "sub-py-run",
		TestID:       "t1",
		Language: profile.LanguageSpec{
			ID:             "py",
			SourceFile:     "main.py",
			CompileEnabled: false,
			RunCmdTpl:      "python3 {src}",
		},
		Profile:    profile.TaskProfile{TaskType: profile.TaskTypeRun},
		WorkDir:    workDir,
		SourcePath: sourcePath,
		IOConfig:   runner.IOConfig{Mode: "stdio"},
		InputPath:  inputPath,
		AnswerPath: answerPath,
		Limits:     spec.ResourceLimit{WallTimeMs: 1000},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(engine.runSpecs) != 1 {
		t.Fatalf("expected 1 run spec, got %d", len(engine.runSpecs))
	}
	runSpec := engine.runSpecs[0]
	if len(runSpec.Cmd) < 2 || runSpec.Cmd[0] != "python3" || runSpec.Cmd[1] != "/work/main.py" {
		t.Fatalf("unexpected python cmd: %v", runSpec.Cmd)
	}
	data, err := os.ReadFile(filepath.Join(workDir, "main.py"))
	if err != nil {
		t.Fatalf("read copied source: %v", err)
	}
	if string(data) != sourceContent {
		t.Fatalf("unexpected copied source: %q", string(data))
	}
}

func TestDispatchRunnerRejectsUnknownLanguage(t *testing.T) {
	r := runner.NewRunner(&fakeEngine{})
	_, err := r.Run(context.Background(), runner.RunRequest{
		SubmissionID: "sub-unknown",
		TestID:       "t1",
		Language: profile.LanguageSpec{
			ID:         "java",
			SourceFile: "Main.java",
			RunCmdTpl:  "java Main",
		},
		Profile:   profile.TaskProfile{TaskType: profile.TaskTypeRun},
		WorkDir:   t.TempDir(),
		IOConfig:  runner.IOConfig{Mode: "stdio"},
		InputPath: filepath.Join(t.TempDir(), "input.txt"),
	})
	if err == nil {
		t.Fatalf("expected unknown language to fail")
	}
}
