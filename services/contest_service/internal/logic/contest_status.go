package logic

import "time"

func deriveContestStatus(rawStatus string, now, startAt, endAt time.Time, freezeMinutes int) string {
	switch rawStatus {
	case "draft":
		return "draft"
	case "ended":
		return "ended"
	}
	if now.Before(startAt) {
		return "published"
	}
	if !now.Before(endAt) {
		return "ended"
	}
	if freezeMinutes > 0 {
		freezeAt := endAt.Add(-time.Duration(freezeMinutes) * time.Minute)
		if !now.Before(freezeAt) {
			return "frozen"
		}
	}
	return "running"
}

func canSubmitByStatus(status string) bool {
	switch status {
	case "published", "running", "frozen":
		return true
	default:
		return false
	}
}
