// Package workspace defines the sandbox directory layout and paths.
package workspace

// Layout describes the filesystem layout for one test execution.
type Layout struct {
	RootDir      string
	WorkDir      string
	SourcePath   string
	BinaryPath   string
	InputPath    string
	OutputPath   string
	AnswerPath   string
	CompileLog   string
	RuntimeLog   string
	TestID       string
	SubmissionID string
}
