package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_service/internal/pmodel"
	"fuzoj/services/rank_service/internal/types"

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

// LeaderboardRepository handles leaderboard storage.
type LeaderboardRepository struct {
	redis    *redis.Redis
	pageTTL  time.Duration
	emptyTTL time.Duration
}

// UpdateApplier applies rank updates.
type UpdateApplier interface {
	ApplyUpdates(ctx context.Context, events []pmodel.RankUpdateEvent) error
}

// RankUpdateMeta holds max version information for a contest.
type RankUpdateMeta struct {
	MaxVersion   int64
	MaxUpdatedAt int64
}

// NewLeaderboardRepository creates a new repository.
func NewLeaderboardRepository(redisClient *redis.Redis, pageTTL, emptyTTL time.Duration) *LeaderboardRepository {
	return &LeaderboardRepository{
		redis:    redisClient,
		pageTTL:  pageTTL,
		emptyTTL: emptyTTL,
	}
}

// ApplyUpdates applies batch updates to leaderboard storage.
func (r *LeaderboardRepository) ApplyUpdates(ctx context.Context, events []pmodel.RankUpdateEvent) error {
	logger := logx.WithContext(ctx)
	if r == nil || r.redis == nil {
		logger.Error("redis is not configured")
		return appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	if len(events) == 0 {
		return nil
	}
	currentVersions := make(map[string]int64)
	contestIDs := uniqueContestIDs(events)
	for _, contestID := range contestIDs {
		versionStr, err := r.redis.HgetCtx(ctx, metaKey(contestID), "version")
		if err != nil && !errors.Is(err, red.Nil) {
			logger.Errorf("load rank meta version failed: %v", err)
			return appErr.Wrapf(err, appErr.CacheError, "load rank meta version failed")
		}
		if versionStr == "" {
			currentVersions[contestID] = 0
			continue
		}
		versionValue, err := strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			logger.Errorf("invalid rank meta version: %v", err)
			return appErr.ValidationError("version", "invalid")
		}
		currentVersions[contestID] = versionValue
	}
	filtered, metaInfo, err := SortAndFilterRankUpdates(events, currentVersions)
	if err != nil {
		logger.Errorf("sort rank updates failed: %v", err)
		return err
	}
	if len(filtered) == 0 {
		return nil
	}
	return r.redis.PipelinedCtx(ctx, func(pipe redis.Pipeliner) error {
		for _, event := range filtered {
			if event.ContestID == "" || event.MemberID == "" {
				logger.Error("contest_id and member_id are required")
				return appErr.ValidationError("contest_id", "required")
			}
			summary := pmodel.LeaderboardSummary{
				MemberID:   event.MemberID,
				SortScore:  event.SortScore,
				ScoreTotal: event.ScoreTotal,
				Penalty:    event.Penalty,
				ACCount:    event.ACCount,
				DetailJSON: event.DetailJSON,
				UpdatedAt:  event.UpdatedAt,
				Version:    event.Version,
			}
			payload, err := json.Marshal(summary)
			if err != nil {
				logger.Errorf("marshal summary failed: %v", err)
				return fmt.Errorf("marshal summary failed: %w", err)
			}
			leaderboardKey := leaderboardKey(event.ContestID)
			detailKey := detailKey(event.ContestID, event.MemberID)
			pipe.ZAdd(ctx, leaderboardKey, red.Z{
				Score:  float64(event.SortScore),
				Member: event.MemberID,
			})
			pipe.HSet(ctx, detailKey, "summary", string(payload))
			if event.ProblemID != "" && event.DetailJSON != "" {
				pipe.HSet(ctx, detailKey, problemField(event.ProblemID), event.DetailJSON)
			}
		}
		for contestID, meta := range metaInfo {
			pipe.HSet(ctx, metaKey(contestID), "version", fmt.Sprint(meta.MaxVersion))
			pipe.HSet(ctx, metaKey(contestID), "updated_at", fmt.Sprint(meta.MaxUpdatedAt))
		}
		return nil
	})
}

// SortAndFilterRankUpdates sorts events by version per contest and filters out stale versions.
func SortAndFilterRankUpdates(events []pmodel.RankUpdateEvent, currentVersion map[string]int64) ([]pmodel.RankUpdateEvent, map[string]RankUpdateMeta, error) {
	type versionedEvent struct {
		version int64
		event   pmodel.RankUpdateEvent
	}
	grouped := make(map[string][]versionedEvent)
	for _, event := range events {
		if event.ContestID == "" {
			return nil, nil, appErr.ValidationError("contest_id", "required")
		}
		versionValue, err := strconv.ParseInt(event.Version, 10, 64)
		if err != nil {
			return nil, nil, appErr.ValidationError("version", "invalid")
		}
		grouped[event.ContestID] = append(grouped[event.ContestID], versionedEvent{
			version: versionValue,
			event:   event,
		})
	}
	filtered := make([]pmodel.RankUpdateEvent, 0, len(events))
	metaInfo := make(map[string]RankUpdateMeta)
	for contestID, items := range grouped {
		sort.Slice(items, func(i, j int) bool {
			return items[i].version < items[j].version
		})
		current := currentVersion[contestID]
		for _, item := range items {
			if item.version <= current {
				continue
			}
			filtered = append(filtered, item.event)
			meta := metaInfo[contestID]
			if item.version > meta.MaxVersion {
				meta.MaxVersion = item.version
			}
			if item.event.UpdatedAt > meta.MaxUpdatedAt {
				meta.MaxUpdatedAt = item.event.UpdatedAt
			}
			metaInfo[contestID] = meta
		}
	}
	return filtered, metaInfo, nil
}

func uniqueContestIDs(events []pmodel.RankUpdateEvent) []string {
	seen := make(map[string]struct{}, len(events))
	ids := make([]string, 0, len(events))
	for _, event := range events {
		if event.ContestID == "" {
			continue
		}
		if _, ok := seen[event.ContestID]; ok {
			continue
		}
		seen[event.ContestID] = struct{}{}
		ids = append(ids, event.ContestID)
	}
	return ids
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
	cacheKey := pageCacheKey(contestID, mode, page, pageSize)
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
	memberIDs, err := r.redis.ZrevrangeCtx(ctx, leaderboardKey, start, stop)
	if err != nil {
		logger.Errorf("load leaderboard failed: %v", err)
		return types.LeaderboardPayload{}, appErr.Wrapf(err, appErr.CacheError, "load leaderboard failed")
	}
	total, err := r.redis.ZcardCtx(ctx, leaderboardKey)
	if err != nil {
		logger.Errorf("load leaderboard total failed: %v", err)
		return types.LeaderboardPayload{}, appErr.Wrapf(err, appErr.CacheError, "load leaderboard total failed")
	}
	entries := make([]types.LeaderboardEntry, 0, len(memberIDs))
	for idx, memberID := range memberIDs {
		summary, err := r.loadSummary(ctx, contestID, memberID)
		if err != nil {
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
	version := r.loadVersion(ctx, contestID)
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

// GetMember returns a member rank entry.
func (r *LeaderboardRepository) GetMember(ctx context.Context, contestID, memberID, mode string) (types.LeaderboardEntry, string, error) {
	logger := logx.WithContext(ctx)
	if r == nil || r.redis == nil {
		logger.Error("redis is not configured")
		return types.LeaderboardEntry{}, "", appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	if contestID == "" || memberID == "" {
		logger.Error("contest_id and member_id are required")
		return types.LeaderboardEntry{}, "", appErr.ValidationError("member_id", "required")
	}
	rank, err := r.redis.ZrevrankCtx(ctx, leaderboardKeyByMode(contestID, mode), memberID)
	if err != nil {
		if errors.Is(err, red.Nil) {
			return types.LeaderboardEntry{}, "", appErr.New(appErr.NotFound).WithMessage("member not found")
		}
		logger.Errorf("load member rank failed: %v", err)
		return types.LeaderboardEntry{}, "", appErr.Wrapf(err, appErr.CacheError, "load member rank failed")
	}
	summary, err := r.loadSummary(ctx, contestID, memberID)
	if err != nil {
		return types.LeaderboardEntry{}, "", err
	}
	if summary == nil {
		return types.LeaderboardEntry{}, "", appErr.New(appErr.NotFound).WithMessage("member not found")
	}
	version := r.loadVersion(ctx, contestID)
	entry := types.LeaderboardEntry{
		MemberId: summary.MemberID,
		Rank:     rank + 1,
		Score:    summary.ScoreTotal,
		Penalty:  summary.Penalty,
		Detail:   summary.DetailJSON,
	}
	return entry, version, nil
}

func (r *LeaderboardRepository) loadSummary(ctx context.Context, contestID, memberID string) (*pmodel.LeaderboardSummary, error) {
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
	var summary pmodel.LeaderboardSummary
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

func pageCacheKey(contestID, mode string, page, pageSize int) string {
	if mode == "" {
		mode = "live"
	}
	return fmt.Sprintf("%s%s:%s:%d:%d", pageCachePrefix, contestID, mode, page, pageSize)
}

func problemField(problemID string) string {
	return "p:" + problemID
}

func ttlSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	return int(ttl.Seconds())
}
