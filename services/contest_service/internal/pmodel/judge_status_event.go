package pmodel

// StatusEventType represents the status event type.
type StatusEventType string

const (
	// StatusEventFinal indicates the final status event.
	StatusEventFinal StatusEventType = "final"
)

// StatusEvent carries judge status updates.
type StatusEvent struct {
	Type      StatusEventType     `json:"type"`
	Status    JudgeStatusResponse `json:"status"`
	CreatedAt int64               `json:"created_at"`
}

// JudgeStatusResponse is the contest-facing final status payload.
type JudgeStatusResponse struct {
	SubmissionID string `json:"submission_id"`
	ContestID    string `json:"contest_id"`
	UserID       string `json:"user_id"`
	ProblemID    int64  `json:"problem_id"`
	Status       string `json:"status"`
	Verdict      string `json:"verdict"`
	Timestamps   struct {
		ReceivedAt int64 `json:"ReceivedAt"`
		FinishedAt int64 `json:"FinishedAt"`
	} `json:"timestamps"`
	CreatedAt int64 `json:"submission_created_at"`
}
