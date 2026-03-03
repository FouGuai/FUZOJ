package statuswriter

// StatusPayload mirrors the submission final status payload.
type StatusPayload struct {
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

type Progress struct {
	TotalTests int `json:"total_tests"`
	DoneTests  int `json:"done_tests"`
}

type SummaryStat struct {
	TotalTimeMs  int64  `json:"TotalTimeMs"`
	MaxMemoryKB  int64  `json:"MaxMemoryKB"`
	TotalScore   int    `json:"TotalScore"`
	FailedTestID string `json:"FailedTestID"`
}

type CompileResult struct {
	OK       bool   `json:"OK"`
	ExitCode int    `json:"ExitCode"`
	TimeMs   int64  `json:"TimeMs"`
	MemoryKB int64  `json:"MemoryKB"`
	Log      string `json:"Log"`
	Error    string `json:"Error"`
}

type TestcaseResult struct {
	TestID     string `json:"TestID"`
	Verdict    string `json:"Verdict"`
	TimeMs     int64  `json:"TimeMs"`
	MemoryKB   int64  `json:"MemoryKB"`
	OutputKB   int64  `json:"OutputKB"`
	ExitCode   int    `json:"ExitCode"`
	RuntimeLog string `json:"RuntimeLog"`
	CheckerLog string `json:"CheckerLog"`
	Stdout     string `json:"Stdout"`
	Stderr     string `json:"Stderr"`
	Score      int    `json:"Score"`
	SubtaskID  string `json:"SubtaskID"`
}

type Timestamps struct {
	ReceivedAt int64 `json:"ReceivedAt"`
	FinishedAt int64 `json:"FinishedAt"`
}
