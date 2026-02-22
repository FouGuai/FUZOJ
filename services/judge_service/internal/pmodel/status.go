package pmodel

import "fuzoj/services/judge_service/internal/sandbox/result"

// JudgeStatusResponse is returned to API clients.
type JudgeStatusResponse struct {
	SubmissionID string                  `json:"submission_id"`
	Status       result.JudgeStatus      `json:"status"`
	Verdict      result.Verdict          `json:"verdict"`
	Score        int                     `json:"score"`
	Language     string                  `json:"language"`
	Summary      result.SummaryStat      `json:"summary"`
	Compile      *result.CompileResult   `json:"compile,omitempty"`
	Tests        []result.TestcaseResult `json:"tests,omitempty"`
	Timestamps   result.Timestamps       `json:"timestamps"`
	Progress     Progress                `json:"progress"`
	ErrorCode    int                     `json:"error_code,omitempty"`
	ErrorMessage string                  `json:"error_message,omitempty"`
}

// Progress represents judge progress.
type Progress struct {
	TotalTests int `json:"total_tests"`
	DoneTests  int `json:"done_tests"`
}
