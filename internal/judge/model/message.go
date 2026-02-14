package model

// JudgeMessage represents the Kafka payload for judge tasks.
type JudgeMessage struct {
	SubmissionID      string   `json:"submission_id"`
	ProblemID         int64    `json:"problem_id"`
	LanguageID        string   `json:"language_id"`
	SourceKey         string   `json:"source_key"`
	SourceHash        string   `json:"source_hash"`
	ContestID         string   `json:"contest_id"`
	UserID            string   `json:"user_id"`
	Priority          int      `json:"priority"`
	ExtraCompileFlags []string `json:"extra_compile_flags"`
}
