package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	rankpb "fuzoj/api/proto/rank"
	appErr "fuzoj/pkg/errors"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	pageCachePrefix   = "contest:lb:page:"
	leaderboardPrefix = "contest:lb:"
	leaderboardFrozen = "contest:lb:frozen:"
	detailPrefix      = "contest:lb:detail:"
	metaPrefix        = "contest:lb:meta:"
)

type leaderboardSummary struct {
	MemberID   string `json:"member_id"`
	SortScore  int64  `json:"sort_score"`
	ScoreTotal int64  `json:"score_total"`
	Penalty    int64  `json:"penalty_total"`
	ACCount    int64  `json:"ac_count"`
	DetailJSON string `json:"detail_json"`
	UpdatedAt  int64  `json:"updated_at"`
	Version    string `json:"version"`
}

// LeaderboardRepository handles leaderboard storage.
type LeaderboardRepository struct {
	redis    *redis.Redis
	pageTTL  time.Duration
	emptyTTL time.Duration
}

// NewLeaderboardRepository creates a new repository.
func NewLeaderboardRepository(redisClient *redis.Redis, pageTTL, emptyTTL time.Duration) *LeaderboardRepository {
	return &LeaderboardRepository{
		redis:    redisClient,
		pageTTL:  pageTTL,
		emptyTTL: emptyTTL,
	}
}

// GetPage returns a leaderboard page.
func (r *LeaderboardRepository) GetPage(ctx context.Context, contestID string, page, pageSize int, mode string) (*rankpb.LeaderboardReply, error) {
	logger := logx.WithContext(ctx)
	if r == nil || r.redis == nil {
		logger.Error("redis is not configured")
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	if contestID == "" {
		logger.Error("contest_id is required")
		return nil, appErr.ValidationError("contest_id", "required")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	cacheKey := pageCacheKey(contestID, mode, page, pageSize)
	if cached, err := r.redis.GetCtx(ctx, cacheKey); err == nil && cached != "" {
		var payload rankpb.LeaderboardReply
		if err := json.Unmarshal([]byte(cached), &payload); err == nil {
			return &payload, nil
		}
	}

	leaderboardKey := leaderboardKeyByMode(contestID, mode)
	start := int64((page - 1) * pageSize)
	stop := start + int64(pageSize) - 1
	memberIDs, err := r.redis.ZrevrangeCtx(ctx, leaderboardKey, start, stop)
	if err != nil {
		logger.Errorf("load leaderboard failed: %v", err)
		return nil, appErr.Wrapf(err, appErr.CacheError, "load leaderboard failed")
	}
	total, err := r.redis.ZcardCtx(ctx, leaderboardKey)
	if err != nil {
		logger.Errorf("load leaderboard total failed: %v", err)
		return nil, appErr.Wrapf(err, appErr.CacheError, "load leaderboard total failed")
	}
	entries := make([]*rankpb.LeaderboardEntry, 0, len(memberIDs))
	for idx, memberID := range memberIDs {
		summary, err := r.loadSummary(ctx, contestID, memberID)
		if err != nil {
			return nil, err
		}
		if summary == nil {
			continue
		}
		entries = append(entries, &rankpb.LeaderboardEntry{
			MemberId:   summary.MemberID,
			Rank:       int64(page-1)*int64(pageSize) + int64(idx) + 1,
			Score:      summary.ScoreTotal,
			Penalty:    summary.Penalty,
			DetailJson: summary.DetailJSON,
		})
	}
	version := r.loadVersion(ctx, contestID)
	payload := &rankpb.LeaderboardReply{
		Items: entries,
		Page: &rankpb.PageInfo{
			Page:     int32(page),
			PageSize: int32(pageSize),
			Total:    int64(total),
		},
		Version: version,
	}
	payloadJSON, err := json.Marshal(payload)
	if err == nil {
		ttl := r.pageTTL
		if total == 0 && r.emptyTTL > 0 {
			ttl = r.emptyTTL
		}
		_ = r.redis.SetexCtx(ctx, cacheKey, string(payloadJSON), ttlSeconds(ttl))
	}
	return payload, nil
}

// GetMember returns a member rank entry.
func (r *LeaderboardRepository) GetMember(ctx context.Context, contestID, memberID, mode string) (*rankpb.MemberRankReply, error) {
	logger := logx.WithContext(ctx)
	if r == nil || r.redis == nil {
		logger.Error("redis is not configured")
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	if contestID == "" || memberID == "" {
		logger.Error("contest_id and member_id are required")
		return nil, appErr.ValidationError("member_id", "required")
	}
	leaderboardKey := leaderboardKeyByMode(contestID, mode)
	rank, err := r.redis.ZrevrankCtx(ctx, leaderboardKey, memberID)
	if err != nil {
		if errors.Is(err, red.Nil) {
			return nil, appErr.New(appErr.NotFound).WithMessage("member not found")
		}
		logger.Errorf("load member rank failed: %v", err)
		return nil, appErr.Wrapf(err, appErr.CacheError, "load member rank failed")
	}
	total, err := r.redis.ZcardCtx(ctx, leaderboardKey)
	if err != nil {
		logger.Errorf("load leaderboard total failed: %v", err)
		return nil, appErr.Wrapf(err, appErr.CacheError, "load leaderboard total failed")
	}
	summary, err := r.loadSummary(ctx, contestID, memberID)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		return nil, appErr.New(appErr.NotFound).WithMessage("member not found")
	}
	version := r.loadVersion(ctx, contestID)
	entry := &rankpb.LeaderboardEntry{
		MemberId:   summary.MemberID,
		Rank:       rank + 1,
		Score:      summary.ScoreTotal,
		Penalty:    summary.Penalty,
		DetailJson: summary.DetailJSON,
	}
	return &rankpb.MemberRankReply{
		Entry:   entry,
		Total:   int64(total),
		Version: version,
	}, nil
}

func (r *LeaderboardRepository) loadSummary(ctx context.Context, contestID, memberID string) (*leaderboardSummary, error) {
	if r.redis == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	val, err := r.redis.HgetCtx(ctx, detailKey(contestID, memberID), "summary")
	if err != nil {
		if errors.Is(err, red.Nil) {
			return nil, nil
		}
		return nil, appErr.Wrapf(err, appErr.CacheError, "load summary failed")
	}
	if val == "" {
		return nil, nil
	}
	var summary leaderboardSummary
	if err := json.Unmarshal([]byte(val), &summary); err != nil {
		return nil, fmt.Errorf("decode summary failed: %w", err)
	}
	return &summary, nil
}

func (r *LeaderboardRepository) loadVersion(ctx context.Context, contestID string) string {
	if r.redis == nil {
		return ""
	}
	val, err := r.redis.HgetCtx(ctx, metaKey(contestID), "version")
	if err != nil {
		return ""
	}
	return val
}

func leaderboardKey(contestID string) string {
	return leaderboardPrefix + contestID
}

func leaderboardKeyByMode(contestID, mode string) string {
	if mode == "frozen" {
		return leaderboardFrozen + contestID
	}
	return leaderboardKey(contestID)
}

func detailKey(contestID, memberID string) string {
	return detailPrefix + contestID + ":" + memberID
}

func metaKey(contestID string) string {
	return metaPrefix + contestID
}

func pageCacheKey(contestID, mode string, page, pageSize int) string {
	if mode == "" {
		mode = "live"
	}
	return fmt.Sprintf("%s%s:%s:%d:%d", pageCachePrefix, contestID, mode, page, pageSize)
}

func ttlSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	return int(ttl.Seconds())
}
