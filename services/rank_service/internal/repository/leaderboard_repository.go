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

var rankApplyScript = redis.NewScript(`
local leaderboardKey = KEYS[1]
local metaKey = KEYS[2]

local eventCount = tonumber(ARGV[1]) or 0
local applyMeta = tonumber(ARGV[2]) or 0
local forceApply = tonumber(ARGV[3]) or 0
local maxResultId = tonumber(ARGV[4]) or 0
local maxVersion = tonumber(ARGV[5]) or 0
local maxUpdatedAt = tonumber(ARGV[6]) or 0
local snapshotAt = tonumber(ARGV[7]) or 0
local detailPrefix = ARGV[8] or ""

local currentResultId = tonumber(redis.call("HGET", metaKey, "result_id") or "0") or 0
local currentVersion = tonumber(redis.call("HGET", metaKey, "version") or "0") or 0

local offset = 8
local stride = 7
local applied = 0

for i = 0, eventCount - 1 do
	local base = offset + i * stride
	local memberId = ARGV[base + 1] or ""
	local sortScore = tonumber(ARGV[base + 2]) or 0
	local summaryJSON = ARGV[base + 3] or ""
	local problemId = ARGV[base + 4] or ""
	local detailJSON = ARGV[base + 5] or ""
	local resultId = tonumber(ARGV[base + 6]) or 0
	local version = tonumber(ARGV[base + 7]) or 0

	local shouldApply = forceApply == 1
	if not shouldApply then
		if resultId > 0 then
			shouldApply = resultId > currentResultId
		elseif version > 0 then
			shouldApply = version > currentVersion
		end
	end

	if shouldApply and memberId ~= "" then
		redis.call("ZADD", leaderboardKey, sortScore, memberId)
		redis.call("HSET", detailPrefix .. memberId, "summary", summaryJSON)
		if problemId ~= "" and detailJSON ~= "" then
			redis.call("HSET", detailPrefix .. memberId, "p:" .. problemId, detailJSON)
		end
		if resultId > currentResultId then
			currentResultId = resultId
		end
		if version > currentVersion then
			currentVersion = version
		end
		applied = applied + 1
	end
end

if applyMeta == 1 then
	if maxResultId > currentResultId then
		currentResultId = maxResultId
	end
	if maxVersion > currentVersion then
		currentVersion = maxVersion
	end
	if currentResultId > 0 then
		redis.call("HSET", metaKey, "result_id", tostring(currentResultId))
	end
	if currentVersion > 0 then
		redis.call("HSET", metaKey, "version", tostring(currentVersion))
	end
	if maxUpdatedAt > 0 then
		redis.call("HSET", metaKey, "updated_at", tostring(maxUpdatedAt))
	end
	if snapshotAt > 0 then
		redis.call("HSET", metaKey, "snapshot_at", tostring(snapshotAt))
	end
end

return applied
`)

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
	MaxResultID  int64
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
	currentResultIDs := make(map[string]int64)
	contestIDs := uniqueContestIDs(events)
	for _, contestID := range contestIDs {
		versionStr, err := r.redis.HgetCtx(ctx, metaKey(contestID), "version")
		if err != nil && !errors.Is(err, red.Nil) {
			logger.Errorf("load rank meta version failed: %v", err)
			return appErr.Wrapf(err, appErr.CacheError, "load rank meta version failed")
		}
		if versionStr == "" {
			currentVersions[contestID] = 0
		} else {
			versionValue, err := strconv.ParseInt(versionStr, 10, 64)
			if err != nil {
				logger.Errorf("invalid rank meta version: %v", err)
				return appErr.ValidationError("version", "invalid")
			}
			currentVersions[contestID] = versionValue
		}
		resultIDStr, err := r.redis.HgetCtx(ctx, metaKey(contestID), "result_id")
		if err != nil && !errors.Is(err, red.Nil) {
			logger.Errorf("load rank meta result id failed: %v", err)
			return appErr.Wrapf(err, appErr.CacheError, "load rank meta result id failed")
		}
		if resultIDStr == "" {
			currentResultIDs[contestID] = 0
		} else {
			resultIDValue, err := strconv.ParseInt(resultIDStr, 10, 64)
			if err != nil {
				logger.Errorf("invalid rank meta result id: %v", err)
				return appErr.ValidationError("result_id", "invalid")
			}
			currentResultIDs[contestID] = resultIDValue
		}
	}
	filtered, metaInfo, err := SortAndFilterRankUpdates(events, currentVersions, currentResultIDs)
	if err != nil {
		logger.Errorf("sort rank updates failed: %v", err)
		return err
	}
	if len(filtered) == 0 {
		return nil
	}
	grouped := make(map[string][]pmodel.RankUpdateEvent)
	for _, event := range filtered {
		if event.ContestID == "" || event.MemberID == "" {
			logger.Error("contest_id and member_id are required")
			return appErr.ValidationError("contest_id", "required")
		}
		grouped[event.ContestID] = append(grouped[event.ContestID], event)
	}
	for contestID, groupedEvents := range grouped {
		if err := r.applyContestEvents(ctx, contestID, groupedEvents, metaInfo[contestID], true, false, 0); err != nil {
			logger.Errorf("apply contest updates failed: %v", err)
			return err
		}
	}
	return nil
}

// SortAndFilterRankUpdates sorts events by result id (preferred) or version per contest and filters out stale updates.
func SortAndFilterRankUpdates(events []pmodel.RankUpdateEvent, currentVersion map[string]int64, currentResultID map[string]int64) ([]pmodel.RankUpdateEvent, map[string]RankUpdateMeta, error) {
	type versionedEvent struct {
		version int64
		event   pmodel.RankUpdateEvent
	}
	type resultEvent struct {
		resultID int64
		event    pmodel.RankUpdateEvent
	}
	groupedLegacy := make(map[string][]versionedEvent)
	groupedResult := make(map[string][]resultEvent)
	for _, event := range events {
		if event.ContestID == "" {
			return nil, nil, appErr.ValidationError("contest_id", "required")
		}
		if event.ResultID > 0 {
			groupedResult[event.ContestID] = append(groupedResult[event.ContestID], resultEvent{
				resultID: event.ResultID,
				event:    event,
			})
			continue
		}
		versionValue, err := strconv.ParseInt(event.Version, 10, 64)
		if err != nil {
			return nil, nil, appErr.ValidationError("version", "invalid")
		}
		groupedLegacy[event.ContestID] = append(groupedLegacy[event.ContestID], versionedEvent{
			version: versionValue,
			event:   event,
		})
	}
	filtered := make([]pmodel.RankUpdateEvent, 0, len(events))
	metaInfo := make(map[string]RankUpdateMeta)
	for contestID, items := range groupedLegacy {
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
	for contestID, items := range groupedResult {
		sort.Slice(items, func(i, j int) bool {
			return items[i].resultID < items[j].resultID
		})
		current := currentResultID[contestID]
		for _, item := range items {
			if item.resultID <= current {
				continue
			}
			filtered = append(filtered, item.event)
			meta := metaInfo[contestID]
			if item.resultID > meta.MaxResultID {
				meta.MaxResultID = item.resultID
			}
			versionValue, err := strconv.ParseInt(item.event.Version, 10, 64)
			if err == nil && versionValue > meta.MaxVersion {
				meta.MaxVersion = versionValue
			}
			if meta.MaxVersion < meta.MaxResultID {
				meta.MaxVersion = meta.MaxResultID
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

// RestoreSnapshotEntries writes snapshot entries into Redis atomically in script mode without advancing meta.
func (r *LeaderboardRepository) RestoreSnapshotEntries(ctx context.Context, contestID string, entries []SnapshotEntry) error {
	if r == nil || r.redis == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	if contestID == "" {
		return appErr.ValidationError("contest_id", "required")
	}
	if len(entries) == 0 {
		return nil
	}
	events := make([]pmodel.RankUpdateEvent, 0, len(entries))
	filteredEntries := make([]SnapshotEntry, 0, len(entries))
	for idx, entry := range entries {
		if entry.MemberID == "" || entry.SummaryJSON == "" {
			continue
		}
		filteredEntries = append(filteredEntries, entry)
		events = append(events, pmodel.RankUpdateEvent{
			ContestID:  contestID,
			MemberID:   entry.MemberID,
			SortScore:  entry.SortScore,
			DetailJSON: entry.DetailJSON,
			// Force mode ignores ordering gates, but keep ResultID unique for deterministic script state.
			ResultID:  int64(idx + 1),
			Version:   "0",
			UpdatedAt: 0,
		})
	}
	if len(events) == 0 {
		return nil
	}
	return r.applyContestEventsWithSummary(ctx, contestID, events, filteredEntries, RankUpdateMeta{}, false, true, 0)
}

// FinalizeSnapshotMeta updates snapshot-related meta fields after all entries are restored.
func (r *LeaderboardRepository) FinalizeSnapshotMeta(ctx context.Context, contestID string, maxResultID, maxVersion, updatedAt, snapshotAt int64) error {
	if r == nil || r.redis == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	if contestID == "" {
		return appErr.ValidationError("contest_id", "required")
	}
	return r.applyContestEvents(ctx, contestID, nil, RankUpdateMeta{
		MaxResultID:  maxResultID,
		MaxVersion:   maxVersion,
		MaxUpdatedAt: updatedAt,
	}, true, false, snapshotAt)
}

func (r *LeaderboardRepository) applyContestEvents(ctx context.Context, contestID string, events []pmodel.RankUpdateEvent, meta RankUpdateMeta, applyMeta, forceApply bool, snapshotAt int64) error {
	return r.applyContestEventsWithSummary(ctx, contestID, events, nil, meta, applyMeta, forceApply, snapshotAt)
}

func (r *LeaderboardRepository) applyContestEventsWithSummary(ctx context.Context, contestID string, events []pmodel.RankUpdateEvent, snapshotEntries []SnapshotEntry, meta RankUpdateMeta, applyMeta, forceApply bool, snapshotAt int64) error {
	if r.redis == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	keys := []string{leaderboardKey(contestID), metaKey(contestID)}
	args := make([]any, 0, 8+len(events)*7)
	args = append(args,
		len(events),
		boolToInt(applyMeta),
		boolToInt(forceApply),
		meta.MaxResultID,
		meta.MaxVersion,
		meta.MaxUpdatedAt,
		snapshotAt,
		detailPrefix+contestID+":",
	)
	for i, event := range events {
		if event.MemberID == "" {
			continue
		}
		summaryJSON := ""
		if i < len(snapshotEntries) && snapshotEntries[i].SummaryJSON != "" {
			summaryJSON = snapshotEntries[i].SummaryJSON
		} else {
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
				return fmt.Errorf("marshal summary failed: %w", err)
			}
			summaryJSON = string(payload)
		}
		versionValue, err := strconv.ParseInt(event.Version, 10, 64)
		if err != nil {
			versionValue = 0
		}
		args = append(args,
			event.MemberID,
			event.SortScore,
			summaryJSON,
			event.ProblemID,
			event.DetailJSON,
			event.ResultID,
			versionValue,
		)
	}
	_, err := r.redis.ScriptRunCtx(ctx, rankApplyScript, keys, args...)
	if err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "apply rank updates with script failed")
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func ttlSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	return int(ttl.Seconds())
}
