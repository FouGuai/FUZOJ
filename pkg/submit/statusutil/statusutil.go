package statusutil

import (
	"strings"
	"time"
)

// IsFinalStatus reports whether status is in final state.
func IsFinalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "finished", "failed":
		return true
	default:
		return false
	}
}

// TTLSeconds converts duration to redis ttl seconds.
func TTLSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}
