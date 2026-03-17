package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_ws_service/internal/pmodel"
	"fuzoj/services/rank_ws_service/internal/types"

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

var rankLoadPageScript = redis.NewScript(`
local leaderboardKey = KEYS[1]
local metaKey = KEYS[2]
local detailPrefix = ARGV[1] or ""
local start = tonumber(ARGV[2]) or 0
local stop = tonumber(ARGV[3]) or -1

local memberIDs = redis.call("ZREVRANGE", leaderboardKey, start, stop)
local total = redis.call("ZCARD", leaderboardKey)
local version = redis.call("HGET", metaKey, "version") or ""

local out = {tostring(total), tostring(version)}
for i = 1, #memberIDs do
	local memberId = memberIDs[i]
	local summary = redis.call("HGET", detailPrefix .. memberId, "summary")
	table.insert(out, memberId)
	table.insert(out, summary or "")
end

return out
`)

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
func (r *LeaderboardRepository) GetPage(ctx context.Context, contestID string, page, pageSize int, mode string) (types.LeaderboardPayload, error) {
	logger := logx.WithContext(ctx)
	if r == nil || r.redis == nil {
		logger.Error("redis is not configured")
		return types.LeaderboardPayload{}, appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	if contestID == "" {
		logger.Error("contest_id is required")
		return types.LeaderboardPayload{}, appErr.ValidationError("contest_id", "required")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	version := r.loadVersion(ctx, contestID)
	cacheKey := pageCacheKey(contestID, mode, page, pageSize, version)
	cached, err := r.redis.GetCtx(ctx, cacheKey)
	if err != nil && !errors.Is(err, red.Nil) {
		logger.Errorf("load leaderboard cache failed: %v", err)
	} else if cached != "" {
		var payload types.LeaderboardPayload
		if err := json.Unmarshal([]byte(cached), &payload); err == nil {
			return payload, nil
		}
	}

	leaderboardKey := leaderboardKeyByMode(contestID, mode)
	start := int64((page - 1) * pageSize)
	stop := start + int64(pageSize) - 1
	total, versionFromScript, rows, err := r.loadPageRows(ctx, contestID, leaderboardKey, start, stop)
	if err != nil {
		logger.Errorf("load leaderboard page rows failed: %v", err)
		return types.LeaderboardPayload{}, err
	}
	entries := make([]types.LeaderboardEntry, 0, len(rows))
	for idx, row := range rows {
		summary, err := decodeSummary(row.summaryJSON)
		if err != nil {
			logger.Errorf("decode summary failed: %v", err)
			return types.LeaderboardPayload{}, err
		}
		if summary == nil {
			continue
		}
		entries = append(entries, types.LeaderboardEntry{
			MemberId: summary.MemberID,
			Rank:     int64(page-1)*int64(pageSize) + int64(idx) + 1,
			Score:    summary.ScoreTotal,
			Penalty:  summary.Penalty,
			Detail:   summary.DetailJSON,
		})
	}
	if versionFromScript != "" {
		version = versionFromScript
	}
	payload := types.LeaderboardPayload{
		Items: entries,
		Page: types.PageInfo{
			Page:     page,
			PageSize: pageSize,
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

type pageRow struct {
	summaryJSON string
}

func (r *LeaderboardRepository) loadPageRows(ctx context.Context, contestID, leaderboardKey string, start, stop int64) (int64, string, []pageRow, error) {
	raw, err := r.redis.ScriptRunCtx(ctx, rankLoadPageScript, []string{leaderboardKey, metaKey(contestID)}, detailPrefix+contestID+":", start, stop)
	if err != nil {
		return 0, "", nil, appErr.Wrapf(err, appErr.CacheError, "load leaderboard page failed")
	}
	values, ok := raw.([]any)
	if !ok || len(values) < 2 {
		return 0, "", nil, appErr.New(appErr.CacheError).WithMessage("invalid leaderboard page response")
	}
	total, err := strconv.ParseInt(fmt.Sprint(values[0]), 10, 64)
	if err != nil {
		return 0, "", nil, fmt.Errorf("parse leaderboard total failed: %w", err)
	}
	version := fmt.Sprint(values[1])
	rows := make([]pageRow, 0, (len(values)-2)/2)
	for i := 2; i+1 < len(values); i += 2 {
		rows = append(rows, pageRow{
			summaryJSON: fmt.Sprint(values[i+1]),
		})
	}
	return total, version, rows, nil
}

func decodeSummary(summaryJSON string) (*pmodel.LeaderboardSummary, error) {
	if summaryJSON == "" {
		return nil, nil
	}
	var summary pmodel.LeaderboardSummary
	if err := json.Unmarshal([]byte(summaryJSON), &summary); err != nil {
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
		if errors.Is(err, red.Nil) {
			return ""
		}
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

func pageCacheKey(contestID, mode string, page, pageSize int, version string) string {
	if mode == "" {
		mode = "live"
	}
	if version == "" {
		version = "0"
	}
	return fmt.Sprintf("%s%s:%s:%d:%d:v%s", pageCachePrefix, contestID, mode, page, pageSize, version)
}

func ttlSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	return int(ttl.Seconds())
}
