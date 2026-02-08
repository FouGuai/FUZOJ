// Package result defines sandbox execution results and verdict mapping.
package result

// JudgeStatus represents the lifecycle state of a submission.
type JudgeStatus string

const (
	StatusPending  JudgeStatus = "Pending"
	StatusRunning  JudgeStatus = "Running"
	StatusFinished JudgeStatus = "Finished"
	StatusFailed   JudgeStatus = "Failed"
)

// Verdict represents the final outcome of execution.
type Verdict string

const (
	VerdictAC  Verdict = "AC"
	VerdictWA  Verdict = "WA"
	VerdictTLE Verdict = "TLE"
	VerdictMLE Verdict = "MLE"
	VerdictOLE Verdict = "OLE"
	VerdictRE  Verdict = "RE"
	VerdictCE  Verdict = "CE"
	VerdictSE  Verdict = "SE"
)

// RunResult captures raw sandbox execution data.
type RunResult struct {
	ExitCode  int
	TimeMs    int64
	MemoryKB  int64
	OutputKB  int64
	Stdout    string
	Stderr    string
	OomKilled bool
}

// CompileResult contains compilation outcomes.
type CompileResult struct {
	OK       bool
	ExitCode int
	TimeMs   int64
	MemoryKB int64
	LogPath  string
	Error    string
}

// TestcaseResult contains per-testcase execution outcomes.
type TestcaseResult struct {
	TestID         string
	Verdict        Verdict
	TimeMs         int64
	MemoryKB       int64
	OutputKB       int64
	ExitCode       int
	RuntimeLogPath string
	CheckerLogPath string
	Stdout         string
	Stderr         string
	Score          int
	SubtaskID      string
}

// SummaryStat captures aggregate statistics across testcases.
type SummaryStat struct {
	TotalTimeMs  int64
	MaxMemoryKB  int64
	TotalScore   int
	FailedTestID string
}

// Timestamps captures submission lifecycle timestamps.
type Timestamps struct {
	ReceivedAt int64
	FinishedAt int64
}

// JudgeResult is the unified response structure for a submission.
type JudgeResult struct {
	SubmissionID string
	Status       JudgeStatus
	Verdict      Verdict
	Score        int
	Language     string
	Compile      *CompileResult
	Tests        []TestcaseResult
	Summary      SummaryStat
	Timestamps   Timestamps
}
