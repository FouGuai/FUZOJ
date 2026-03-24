package score

import "time"

// ICPCPenalty calculates ICPC-style penalty in seconds.
func ICPCPenalty(startAt time.Time, submitAt time.Time, wrongCount int) int64 {
	return ICPCPenaltyWithMinutes(startAt, submitAt, wrongCount, 20)
}

// ICPCPenaltyWithMinutes calculates ICPC-style penalty in seconds with configurable penalty minutes.
func ICPCPenaltyWithMinutes(startAt time.Time, submitAt time.Time, wrongCount int, penaltyMinutes int) int64 {
	if penaltyMinutes <= 0 {
		penaltyMinutes = 20
	}
	if startAt.IsZero() || submitAt.IsZero() {
		return 0
	}
	base := submitAt.Sub(startAt).Seconds()
	if base < 0 {
		base = 0
	}
	return int64(base) + int64(wrongCount)*int64(penaltyMinutes)*60
}

// SortScore computes sort score for ICPC leaderboard.
func SortScore(acCount int64, penaltyTotal int64) int64 {
	return acCount*1_000_000_000_000 - penaltyTotal
}
