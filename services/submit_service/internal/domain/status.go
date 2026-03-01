package domain

// JudgeStatusPayload is the persisted status payload for submissions.
type JudgeStatusPayload struct {
	SubmissionID string           `json:"submission_id"`
	Status       string           `json:"status"`
	Verdict      string           `json:"verdict"`
	Score        int              `json:"score"`
	Language     string           `json:"language"`
	Summary      SummaryStat      `json:"summary"`
	Compile      *CompileResult   `json:"compile,omitempty"`
	Tests        []TestcaseResult `json:"tests,omitempty"`
	Timestamps   Timestamps       `json:"timestamps"`
	Progress     Progress         `json:"progress"`
	ErrorCode    int              `json:"error_code,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
}

// Progress represents judge progress.
type Progress struct {
	TotalTests int `json:"total_tests"`
	DoneTests  int `json:"done_tests"`
}

// SummaryStat captures aggregate statistics across testcases.
type SummaryStat struct {
	TotalTimeMs  int64  `json:"TotalTimeMs"`
	MaxMemoryKB  int64  `json:"MaxMemoryKB"`
	TotalScore   int    `json:"TotalScore"`
	FailedTestID string `json:"FailedTestID"`
}

// CompileResult contains compilation outcomes.
type CompileResult struct {
	OK       bool   `json:"OK"`
	ExitCode int    `json:"ExitCode"`
	TimeMs   int64  `json:"TimeMs"`
	MemoryKB int64  `json:"MemoryKB"`
	Log      string `json:"Log"`
	Error    string `json:"Error"`
}

// TestcaseResult contains per-testcase execution outcomes.
type TestcaseResult struct {
	TestID         string `json:"TestID"`
	Verdict        string `json:"Verdict"`
	TimeMs         int64  `json:"TimeMs"`
	MemoryKB       int64  `json:"MemoryKB"`
	OutputKB       int64  `json:"OutputKB"`
	ExitCode       int    `json:"ExitCode"`
	RuntimeLogPath string `json:"RuntimeLogPath"`
	CheckerLogPath string `json:"CheckerLogPath"`
	Stdout         string `json:"Stdout"`
	Stderr         string `json:"Stderr"`
	Score          int    `json:"Score"`
	SubtaskID      string `json:"SubtaskID"`
}

// Timestamps captures submission lifecycle timestamps.
type Timestamps struct {
	ReceivedAt int64 `json:"ReceivedAt"`
	FinishedAt int64 `json:"FinishedAt"`
}

const (
	StatusPending   = "Pending"
	StatusCompiling = "Compiling"
	StatusRunning   = "Running"
	StatusJudging   = "Judging"
	StatusFinished  = "Finished"
	StatusFailed    = "Failed"
)

// StatusEvent carries status updates for async processing.
type StatusEvent struct {
	Type      string             `json:"type"`
	Status    JudgeStatusPayload `json:"status"`
	CreatedAt int64              `json:"created_at"`
}

const StatusEventFinal = "final"
