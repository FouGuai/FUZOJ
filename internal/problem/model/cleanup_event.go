package model

import "time"

const (
	ProblemCleanupEventDeleted = "problem.deleted"
)

// ProblemCleanupEvent represents a cleanup request for problem data.
type ProblemCleanupEvent struct {
	EventType   string    `json:"event_type"`
	ProblemID   int64     `json:"problem_id"`
	Bucket      string    `json:"bucket"`
	Prefix      string    `json:"prefix"`
	RequestedAt time.Time `json:"requested_at"`
}
