package repository

// MetaKey exposes the rank meta key.
func MetaKey(contestID string) string {
	return metaKey(contestID)
}

// MetaPrefix exposes the meta key prefix.
func MetaPrefix() string {
	return metaPrefix
}

// ContestIDFromMetaKey parses contest id from a meta key.
func ContestIDFromMetaKey(key string) string {
	return parseContestIDFromMetaKey(key)
}

// LeaderboardKey exposes the leaderboard zset key.
func LeaderboardKey(contestID string) string {
	return leaderboardKey(contestID)
}

// DetailKey exposes the detail hash key.
func DetailKey(contestID, memberID string) string {
	return detailKey(contestID, memberID)
}
