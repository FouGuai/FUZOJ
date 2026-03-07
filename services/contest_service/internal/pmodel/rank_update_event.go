package pmodel

// RankUpdateEvent represents a pre-computed leaderboard update payload.
type RankUpdateEvent struct {
	ContestID  string `json:"contest_id"`
	MemberID   string `json:"member_id"`
	ProblemID  string `json:"problem_id"`
	SortScore  int64  `json:"sort_score"`
	ScoreTotal int64  `json:"score_total"`
	Penalty    int64  `json:"penalty_total"`
	ACCount    int64  `json:"ac_count"`
	DetailJSON string `json:"detail_json"`
	Version    string `json:"version"`
	ResultID   int64  `json:"result_id"`
	UpdatedAt  int64  `json:"updated_at"`
}
