package tests

import (
	"testing"

	"fuzoj/services/rank_service/internal/repository"
)

func TestRankKeys_UseSameClusterSlotTag(t *testing.T) {
	contestID := "contest-1"
	memberID := "member-1"

	if got := repository.LeaderboardKey(contestID); got != "contest:lb:{contest-1}" {
		t.Fatalf("unexpected leaderboard key: %s", got)
	}
	if got := repository.MetaKey(contestID); got != "contest:lb:meta:{contest-1}" {
		t.Fatalf("unexpected meta key: %s", got)
	}
	if got := repository.DetailKey(contestID, memberID); got != "contest:lb:detail:{contest-1}:member-1" {
		t.Fatalf("unexpected detail key: %s", got)
	}
}

func TestContestIDFromMetaKey(t *testing.T) {
	if got := repository.ContestIDFromMetaKey("contest:lb:meta:{contest-2}"); got != "contest-2" {
		t.Fatalf("unexpected contest id: %s", got)
	}
	if got := repository.ContestIDFromMetaKey("contest:lb:meta:contest-legacy"); got != "contest-legacy" {
		t.Fatalf("unexpected legacy contest id: %s", got)
	}
}
