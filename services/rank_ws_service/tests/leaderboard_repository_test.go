package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"fuzoj/services/rank_ws_service/internal/pmodel"
	"fuzoj/services/rank_ws_service/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestLeaderboardRepository_GetPageBypassesStaleCacheAfterVersionChange(t *testing.T) {
	repo, cache := newLeaderboardRepoForTest(t)
	ctx := context.Background()

	if err := seedMember(cache, "c1", "m1", 10, 100, "1"); err != nil {
		t.Fatalf("seed initial member failed: %v", err)
	}

	first, err := repo.GetPage(ctx, "c1", 1, 50, "")
	if err != nil {
		t.Fatalf("get initial page failed: %v", err)
	}
	if first.Version != "1" {
		t.Fatalf("expected version=1, got %s", first.Version)
	}
	if len(first.Items) != 1 || first.Items[0].Score != 10 {
		t.Fatalf("unexpected initial page: %+v", first.Items)
	}

	if err := seedMember(cache, "c1", "m1", 20, 200, "2"); err != nil {
		t.Fatalf("seed updated member failed: %v", err)
	}

	second, err := repo.GetPage(ctx, "c1", 1, 50, "")
	if err != nil {
		t.Fatalf("get updated page failed: %v", err)
	}
	if second.Version != "2" {
		t.Fatalf("expected version=2, got %s", second.Version)
	}
	if len(second.Items) != 1 || second.Items[0].Score != 20 {
		t.Fatalf("expected updated score=20, got %+v", second.Items)
	}
}

func seedMember(cache *redis.Redis, contestID, memberID string, score, sortScore int64, version string) error {
	ctx := context.Background()
	summary, err := json.Marshal(pmodel.LeaderboardSummary{
		MemberID:   memberID,
		SortScore:  sortScore,
		ScoreTotal: score,
		Version:    version,
	})
	if err != nil {
		return err
	}
	if _, err := cache.ZaddCtx(ctx, "contest:lb:"+contestID, sortScore, memberID); err != nil {
		return err
	}
	if err := cache.HsetCtx(ctx, "contest:lb:detail:"+contestID+":"+memberID, "summary", string(summary)); err != nil {
		return err
	}
	if err := cache.HsetCtx(ctx, "contest:lb:meta:"+contestID, "version", version); err != nil {
		return err
	}
	return nil
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
