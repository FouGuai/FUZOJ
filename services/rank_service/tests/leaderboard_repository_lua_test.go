package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"fuzoj/services/rank_service/internal/pmodel"
	"fuzoj/services/rank_service/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestLeaderboardRepository_ApplyUpdates_ResultIDAdvancesVersion(t *testing.T) {
	repo, cache := newLeaderboardRepoForTest(t)
	ctx := context.Background()

	err := repo.ApplyUpdates(ctx, []pmodel.RankUpdateEvent{
		{
			ContestID:  "c1",
			MemberID:   "m1",
			SortScore:  10,
			ScoreTotal: 1,
			Version:    "1",
			ResultID:   1,
			UpdatedAt:  100,
		},
		{
			ContestID:  "c1",
			MemberID:   "m2",
			SortScore:  20,
			ScoreTotal: 2,
			Version:    "1",
			ResultID:   2,
			UpdatedAt:  101,
		},
	})
	if err != nil {
		t.Fatalf("apply updates failed: %v", err)
	}

	resultID, err := cache.HgetCtx(ctx, repository.MetaKey("c1"), "result_id")
	if err != nil {
		t.Fatalf("load result id failed: %v", err)
	}
	if resultID != "2" {
		t.Fatalf("expected result_id=2, got %s", resultID)
	}

	version, err := cache.HgetCtx(ctx, repository.MetaKey("c1"), "version")
	if err != nil {
		t.Fatalf("load version failed: %v", err)
	}
	if version != "2" {
		t.Fatalf("expected version=2, got %s", version)
	}
}

func TestLeaderboardRepository_ApplyUpdates_IgnoresStaleResultID(t *testing.T) {
	repo, _ := newLeaderboardRepoForTest(t)
	ctx := context.Background()

	if err := repo.ApplyUpdates(ctx, []pmodel.RankUpdateEvent{
		{
			ContestID:  "c1",
			MemberID:   "m1",
			SortScore:  30,
			ScoreTotal: 3,
			Version:    "3",
			ResultID:   3,
			UpdatedAt:  103,
		},
	}); err != nil {
		t.Fatalf("apply updates failed: %v", err)
	}

	if err := repo.ApplyUpdates(ctx, []pmodel.RankUpdateEvent{
		{
			ContestID:  "c1",
			MemberID:   "m1",
			SortScore:  999,
			ScoreTotal: 999,
			Version:    "4",
			ResultID:   2,
			UpdatedAt:  104,
		},
	}); err != nil {
		t.Fatalf("apply stale updates failed: %v", err)
	}

	entry, _, err := repo.GetMember(ctx, "c1", "m1", "")
	if err != nil {
		t.Fatalf("get member failed: %v", err)
	}
	if entry.Score != 3 {
		t.Fatalf("expected score=3 after stale update, got %d", entry.Score)
	}
}

func TestLeaderboardRepository_ApplyUpdates_OutOfOrderDifferentMembersNotDropped(t *testing.T) {
	repo, _ := newLeaderboardRepoForTest(t)
	ctx := context.Background()

	if err := repo.ApplyUpdates(ctx, []pmodel.RankUpdateEvent{
		{
			ContestID:  "c1",
			MemberID:   "m1",
			SortScore:  100,
			ScoreTotal: 10,
			Version:    "10",
			ResultID:   100,
			UpdatedAt:  200,
		},
		{
			ContestID:  "c1",
			MemberID:   "m2",
			SortScore:  90,
			ScoreTotal: 9,
			Version:    "9",
			ResultID:   99,
			UpdatedAt:  199,
		},
	}); err != nil {
		t.Fatalf("apply updates failed: %v", err)
	}

	first, _, err := repo.GetMember(ctx, "c1", "m1", "")
	if err != nil {
		t.Fatalf("get member m1 failed: %v", err)
	}
	second, _, err := repo.GetMember(ctx, "c1", "m2", "")
	if err != nil {
		t.Fatalf("get member m2 failed: %v", err)
	}
	if first.Score != 10 {
		t.Fatalf("expected m1 score=10, got %d", first.Score)
	}
	if second.Score != 9 {
		t.Fatalf("expected m2 score=9, got %d", second.Score)
	}
}

func TestLeaderboardRepository_RestoreSnapshotAndFinalizeMeta(t *testing.T) {
	repo, cache := newLeaderboardRepoForTest(t)
	ctx := context.Background()

	summary := pmodel.LeaderboardSummary{
		MemberID:   "m1",
		SortScore:  40,
		ScoreTotal: 4,
		Penalty:    10,
		ACCount:    1,
		DetailJSON: "{}",
		Version:    "7",
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary failed: %v", err)
	}

	if err := repo.RestoreSnapshotEntries(ctx, "c1", []repository.SnapshotEntry{
		{
			MemberID:    "m1",
			SortScore:   40,
			DetailJSON:  "{}",
			SummaryJSON: string(summaryJSON),
		},
	}); err != nil {
		t.Fatalf("restore snapshot entries failed: %v", err)
	}

	if version, err := cache.HgetCtx(ctx, repository.MetaKey("c1"), "version"); err == nil && version != "" {
		t.Fatalf("expected empty meta version before finalize, got %s", version)
	}

	if err := repo.FinalizeSnapshotMeta(ctx, "c1", 9, 7, 200, 200); err != nil {
		t.Fatalf("finalize snapshot meta failed: %v", err)
	}

	resultID, err := cache.HgetCtx(ctx, repository.MetaKey("c1"), "result_id")
	if err != nil {
		t.Fatalf("load result id failed: %v", err)
	}
	if resultID != "9" {
		t.Fatalf("expected result_id=9, got %s", resultID)
	}
}

func TestLeaderboardRepository_GetPageBypassesStaleEmptyCacheAfterUpdate(t *testing.T) {
	repo, _ := newLeaderboardRepoForTest(t)
	ctx := context.Background()

	initial, err := repo.GetPage(ctx, "c1", 1, 50, "")
	if err != nil {
		t.Fatalf("get initial page failed: %v", err)
	}
	if initial.Page.Total != 0 {
		t.Fatalf("expected empty leaderboard, got total=%d", initial.Page.Total)
	}

	if err := repo.ApplyUpdates(ctx, []pmodel.RankUpdateEvent{
		{
			ContestID:  "c1",
			MemberID:   "m1",
			SortScore:  10,
			ScoreTotal: 1,
			Version:    "1",
			ResultID:   1,
			UpdatedAt:  100,
		},
	}); err != nil {
		t.Fatalf("apply updates failed: %v", err)
	}

	updated, err := repo.GetPage(ctx, "c1", 1, 50, "")
	if err != nil {
		t.Fatalf("get updated page failed: %v", err)
	}
	if updated.Page.Total != 1 {
		t.Fatalf("expected total=1 after update, got %d", updated.Page.Total)
	}
	if len(updated.Items) != 1 || updated.Items[0].MemberId != "m1" {
		t.Fatalf("unexpected leaderboard items: %+v", updated.Items)
	}
}

func newLeaderboardRepoForTest(t *testing.T) (*repository.LeaderboardRepository, *redis.Redis) {
	t.Helper()
	mini := miniredis.RunT(t)
	t.Cleanup(mini.Close)

	cache, err := redis.NewRedis(redis.RedisConf{
		Host: mini.Addr(),
		Type: "node",
	})
	if err != nil {
		t.Fatalf("new redis failed: %v", err)
	}
	return repository.NewLeaderboardRepository(cache, time.Second, time.Second), cache
}
