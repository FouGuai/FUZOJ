package score

import "time"

// ICPCPenalty calculates ICPC-style penalty in seconds.
func ICPCPenalty(startAt time.Time, submitAt time.Time, wrongCount int) int64 {
	if startAt.IsZero() || submitAt.IsZero() {
		return 0
	}
	base := submitAt.Sub(startAt).Seconds()
	if base < 0 {
		base = 0
	}
	return int64(base) + int64(wrongCount)*20*60
}

// SortScore computes sort score for ICPC leaderboard.
func SortScore(acCount int64, penaltyTotal int64) int64 {
	return acCount*1_000_000_000_000 - penaltyTotal
}
