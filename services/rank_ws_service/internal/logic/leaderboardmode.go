package logic

import (
	appErr "fuzoj/pkg/errors"
)

const (
	LeaderboardModeLive   = "live"
	LeaderboardModeFrozen = "frozen"
)

// NormalizeLeaderboardMode validates and normalizes leaderboard mode.
func NormalizeLeaderboardMode(mode string) (string, error) {
	if mode == "" {
		return LeaderboardModeLive, nil
	}
	if mode == LeaderboardModeLive || mode == LeaderboardModeFrozen {
		return mode, nil
	}
	return "", appErr.ValidationError("mode", "invalid")
}
