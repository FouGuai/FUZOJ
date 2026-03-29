package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"fuzoj/pkg/contest/score"
	"fuzoj/services/rank_service/internal/pmodel"
	"fuzoj/services/rank_service/internal/repository"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Snapshotter periodically persists rank snapshots.
type Snapshotter struct {
	repo           *repository.SnapshotRepository
	leaderboard    *repository.LeaderboardRepository
	mainSummary    *repository.MainSummaryRepository
	redis          *redis.Redis
	interval       time.Duration
	pageSize       int
	batchSize      int
	cacheTimeout   time.Duration
	dbTimeout      time.Duration
	recoverOnStart bool
	recovery       RecoveryOptions
	stopCh         chan struct{}
	running        int32
}

// RecoveryOptions defines startup recovery behavior.
type RecoveryOptions struct {
	KafkaCatchupEnabled      bool
	KafkaCatchupWindow       time.Duration
	VerifyStrict             bool
	MainTableFallbackEnabled bool
	RebuildBatchSize         int
}

func normalizeRecoveryOptions(options RecoveryOptions) RecoveryOptions {
	if !options.KafkaCatchupEnabled &&
		!options.MainTableFallbackEnabled &&
		options.KafkaCatchupWindow <= 0 &&
		options.RebuildBatchSize <= 0 &&
		!options.VerifyStrict {
		options.KafkaCatchupEnabled = true
		options.MainTableFallbackEnabled = true
	}
	if options.KafkaCatchupWindow <= 0 {
		options.KafkaCatchupWindow = 60 * time.Second
	}
	if options.RebuildBatchSize <= 0 {
		options.RebuildBatchSize = 500
	}
	return options
}

func NewSnapshotter(repo *repository.SnapshotRepository, leaderboard *repository.LeaderboardRepository, mainSummary *repository.MainSummaryRepository, redisClient *redis.Redis, interval time.Duration, pageSize, batchSize int, cacheTimeout, dbTimeout time.Duration, recoverOnStart bool, recovery RecoveryOptions) *Snapshotter {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if pageSize <= 0 {
		pageSize = 500
	}
	if batchSize <= 0 {
		batchSize = 500
	}
	return &Snapshotter{
		repo:           repo,
		leaderboard:    leaderboard,
		mainSummary:    mainSummary,
		redis:          redisClient,
		interval:       interval,
		pageSize:       pageSize,
		batchSize:      batchSize,
		cacheTimeout:   cacheTimeout,
		dbTimeout:      dbTimeout,
		recoverOnStart: recoverOnStart,
		recovery:       normalizeRecoveryOptions(recovery),
		stopCh:         make(chan struct{}),
	}
}

func (s *Snapshotter) Start(ctx context.Context) {
	if s == nil {
		return
	}
	logger := logx.WithContext(ctx)
	logger.Info("rank snapshotter started")
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			logger.Info("rank snapshotter stopped")
			return
		case <-ticker.C:
			s.run(ctx)
		}
	}
}

func (s *Snapshotter) Stop() {
	if s == nil {
		return
	}
	close(s.stopCh)
}

func (s *Snapshotter) Recover(ctx context.Context) {
	if s == nil || s.repo == nil || s.redis == nil || !s.recoverOnStart {
		return
	}
	logger := logx.WithContext(ctx)
	ctxDB := withTimeout(ctx, s.dbTimeout)
	defer ctxDB.cancel()
	metas, err := s.repo.ListLatestReadySnapshotMetas(ctxDB.ctx)
	if err != nil || len(metas) == 0 {
		if err != nil {
			logger.Errorf("load rank snapshot metas failed: %v", err)
		}
		return
	}
	for _, meta := range metas {
		if meta.ContestID == "" {
			continue
		}
		watermark, err := s.loadRedisWatermark(ctx, meta.ContestID)
		if err != nil {
			logger.Errorf("check rank meta failed: %v", err)
			continue
		}
		if !shouldRestoreFromSnapshot(meta, watermark) {
			continue
		}
		if err := s.restoreSnapshot(ctx, meta); err != nil {
			logger.Errorf("restore rank snapshot failed: %v", err)
		}
	}
}

// CatchupAndFallback waits for Kafka catchup window and reconciles with main table.
func (s *Snapshotter) CatchupAndFallback(ctx context.Context) {
	if s == nil || s.repo == nil || s.redis == nil || !s.recoverOnStart {
		return
	}
	logger := logx.WithContext(ctx)
	if s.recovery.KafkaCatchupEnabled && s.recovery.KafkaCatchupWindow > 0 {
		logger.Infof("rank recovery kafka catchup window started: %s", s.recovery.KafkaCatchupWindow)
		timer := time.NewTimer(s.recovery.KafkaCatchupWindow)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}

	ctxDB := withTimeout(ctx, s.dbTimeout)
	metas, err := s.repo.ListLatestReadySnapshotMetas(ctxDB.ctx)
	ctxDB.cancel()
	if err != nil {
		logger.Errorf("load rank snapshot metas for reconciliation failed: %v", err)
		return
	}
	for _, meta := range metas {
		if meta.ContestID == "" {
			continue
		}
		ok, err := s.verifyContestFromMain(ctx, meta.ContestID)
		if err != nil {
			logger.Errorf("verify recovered rank contest failed, contest_id=%s err=%v", meta.ContestID, err)
			continue
		}
		if ok {
			continue
		}
		if !s.recovery.MainTableFallbackEnabled {
			logger.Errorf("rank recovery verification mismatch and fallback disabled, contest_id=%s", meta.ContestID)
			continue
		}
		logger.Infof("rank recovery fallback started, contest_id=%s", meta.ContestID)
		if err := s.rebuildContestFromMain(ctx, meta.ContestID); err != nil {
			logger.Errorf("rank recovery fallback failed, contest_id=%s err=%v", meta.ContestID, err)
			continue
		}
		ok, err = s.verifyContestFromMain(ctx, meta.ContestID)
		if err != nil {
			logger.Errorf("verify rank recovery fallback failed, contest_id=%s err=%v", meta.ContestID, err)
			continue
		}
		if !ok {
			logger.Errorf("rank recovery fallback still mismatched, contest_id=%s", meta.ContestID)
			continue
		}
		logger.Infof("rank recovery fallback completed, contest_id=%s", meta.ContestID)
	}
}

func (s *Snapshotter) run(ctx context.Context) {
	if s == nil || s.repo == nil || s.redis == nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&s.running, 0)

	logger := logx.WithContext(ctx)
	cursor := uint64(0)
	prefix := repository.MetaPrefix()
	for {
		ctxCache := withTimeout(ctx, s.cacheTimeout)
		keys, next, err := s.redis.ScanCtx(ctxCache.ctx, cursor, prefix+"*", 200)
		ctxCache.cancel()
		if err != nil {
			logger.Errorf("scan rank meta failed: %v", err)
			return
		}
		for _, key := range keys {
			contestID := repository.ContestIDFromMetaKey(key)
			if contestID == "" {
				continue
			}
			if err := s.snapshotContest(ctx, contestID); err != nil {
				logger.Errorf("snapshot contest failed: %v", err)
			}
		}
		if next == 0 {
			break
		}
		cursor = next
	}
}

func (s *Snapshotter) snapshotContest(ctx context.Context, contestID string) error {
	logger := logx.WithContext(ctx)
	ctxCache := withTimeout(ctx, s.cacheTimeout)
	metaVals, err := s.redis.HmgetCtx(ctxCache.ctx, repository.MetaKey(contestID), "updated_at", "snapshot_at", "result_id", "version")
	ctxCache.cancel()
	if err != nil {
		return err
	}
	updatedAt := parseInt64(metaVals, 0)
	lastSnapshotAt := parseInt64(metaVals, 1)
	lastResultID := parseInt64(metaVals, 2)
	lastVersion := parseInt64(metaVals, 3)
	if updatedAt == 0 || updatedAt <= lastSnapshotAt {
		return nil
	}

	ctxCache = withTimeout(ctx, s.cacheTimeout)
	total, err := s.redis.ZcardCtx(ctxCache.ctx, repository.LeaderboardKey(contestID))
	ctxCache.cancel()
	if err != nil {
		return err
	}
	if total == 0 {
		return nil
	}

	total64 := int64(total)
	snapshotAt := time.Now()
	ctxDB := withTimeout(ctx, s.dbTimeout)
	snapshotID, err := s.repo.CreateSnapshotMeta(ctxDB.ctx, repository.SnapshotMeta{
		ContestID:    contestID,
		SnapshotAt:   snapshotAt,
		LastResultID: lastResultID,
		LastVersion:  lastVersion,
		Total:        total64,
		Status:       "writing",
	})
	ctxDB.cancel()
	if err != nil {
		return err
	}

	var start int64 = 0
	for start < total64 {
		stop := start + int64(s.pageSize) - 1
		ctxCache = withTimeout(ctx, s.cacheTimeout)
		pairs, err := s.redis.ZrevrangeWithScoresCtx(ctxCache.ctx, repository.LeaderboardKey(contestID), start, stop)
		ctxCache.cancel()
		if err != nil {
			return err
		}
		if len(pairs) == 0 {
			break
		}

		summaries := make([]*red.StringCmd, len(pairs))
		ctxCache = withTimeout(ctx, s.cacheTimeout)
		err = s.redis.PipelinedCtx(ctxCache.ctx, func(pipe redis.Pipeliner) error {
			for i, pair := range pairs {
				memberID := pair.Key
				if memberID == "" {
					continue
				}
				summaries[i] = pipe.HGet(ctxCache.ctx, repository.DetailKey(contestID, memberID), "summary")
			}
			return nil
		})
		ctxCache.cancel()
		if err != nil {
			return err
		}

		entries := make([]repository.SnapshotEntry, 0, len(pairs))
		for i, pair := range pairs {
			memberID := pair.Key
			if memberID == "" {
				continue
			}
			cmd := summaries[i]
			if cmd == nil {
				continue
			}
			summaryJSON, err := cmd.Result()
			if err != nil || summaryJSON == "" {
				continue
			}
			var summary pmodel.LeaderboardSummary
			if err := json.Unmarshal([]byte(summaryJSON), &summary); err != nil {
				logger.Errorf("decode rank summary failed: %v", err)
				continue
			}
			entries = append(entries, repository.SnapshotEntry{
				SnapshotID:  snapshotID,
				MemberID:    memberID,
				Rank:        start + int64(i) + 1,
				SortScore:   int64(pair.Score),
				ScoreTotal:  summary.ScoreTotal,
				Penalty:     summary.Penalty,
				ACCount:     summary.ACCount,
				DetailJSON:  summary.DetailJSON,
				SummaryJSON: summaryJSON,
			})
		}
		if err := s.insertEntries(ctx, entries); err != nil {
			return err
		}
		start += int64(len(pairs))
	}

	ctxDB = withTimeout(ctx, s.dbTimeout)
	if err := s.repo.MarkSnapshotReady(ctxDB.ctx, snapshotID); err != nil {
		ctxDB.cancel()
		return err
	}
	ctxDB.cancel()

	ctxCache = withTimeout(ctx, s.cacheTimeout)
	_ = s.redis.PipelinedCtx(ctxCache.ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctxCache.ctx, repository.MetaKey(contestID), "snapshot_at", strconv.FormatInt(snapshotAt.Unix(), 10))
		return nil
	})
	ctxCache.cancel()

	logger.Infof("rank snapshot saved contest_id=%s total=%d", contestID, total)
	return nil
}

func (s *Snapshotter) insertEntries(ctx context.Context, entries []repository.SnapshotEntry) error {
	if len(entries) == 0 {
		return nil
	}
	for start := 0; start < len(entries); start += s.batchSize {
		end := start + s.batchSize
		if end > len(entries) {
			end = len(entries)
		}
		ctxDB := withTimeout(ctx, s.dbTimeout)
		err := s.repo.InsertSnapshotEntries(ctxDB.ctx, entries[start:end])
		ctxDB.cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Snapshotter) restoreSnapshot(ctx context.Context, meta repository.SnapshotMeta) error {
	logger := logx.WithContext(ctx)
	if s.leaderboard == nil {
		return nil
	}
	var lastRank int64 = 0
	for {
		ctxDB := withTimeout(ctx, s.dbTimeout)
		entries, err := s.repo.ListSnapshotEntriesAfterRank(ctxDB.ctx, meta.ID, lastRank, s.pageSize)
		ctxDB.cancel()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			break
		}
		ctxCache := withTimeout(ctx, s.cacheTimeout)
		if err := s.leaderboard.RestoreSnapshotEntries(ctxCache.ctx, meta.ContestID, entries); err != nil {
			ctxCache.cancel()
			return err
		}
		ctxCache.cancel()
		lastRank = entries[len(entries)-1].Rank
	}

	ctxCache := withTimeout(ctx, s.cacheTimeout)
	_ = s.leaderboard.FinalizeSnapshotMeta(
		ctxCache.ctx,
		meta.ContestID,
		meta.LastResultID,
		meta.LastVersion,
		meta.SnapshotAt.Unix(),
		meta.SnapshotAt.Unix(),
	)
	ctxCache.cancel()

	logger.Infof("rank snapshot restored contest_id=%s", meta.ContestID)
	return nil
}

type redisWatermark struct {
	Exists     bool
	ResultID   int64
	Version    int64
	SnapshotAt int64
	Total      int64
}

func (s *Snapshotter) loadRedisWatermark(ctx context.Context, contestID string) (redisWatermark, error) {
	if s.redis == nil {
		return redisWatermark{}, fmt.Errorf("redis is not configured")
	}
	ctxCache := withTimeout(ctx, s.cacheTimeout)
	exists, err := s.redis.ExistsCtx(ctxCache.ctx, repository.MetaKey(contestID))
	ctxCache.cancel()
	if err != nil {
		return redisWatermark{}, err
	}
	if !exists {
		return redisWatermark{}, nil
	}
	ctxCache = withTimeout(ctx, s.cacheTimeout)
	metaVals, err := s.redis.HmgetCtx(ctxCache.ctx, repository.MetaKey(contestID), "result_id", "version", "snapshot_at")
	ctxCache.cancel()
	if err != nil {
		return redisWatermark{}, err
	}
	ctxCache = withTimeout(ctx, s.cacheTimeout)
	total, err := s.redis.ZcardCtx(ctxCache.ctx, repository.LeaderboardKey(contestID))
	ctxCache.cancel()
	if err != nil {
		return redisWatermark{}, err
	}
	return redisWatermark{
		Exists:     true,
		ResultID:   parseInt64(metaVals, 0),
		Version:    parseInt64(metaVals, 1),
		SnapshotAt: parseInt64(metaVals, 2),
		Total:      int64(total),
	}, nil
}

func shouldRestoreFromSnapshot(meta repository.SnapshotMeta, watermark redisWatermark) bool {
	if !watermark.Exists {
		return true
	}
	if watermark.ResultID < meta.LastResultID {
		return true
	}
	if watermark.Version < meta.LastVersion {
		return true
	}
	if watermark.SnapshotAt < meta.SnapshotAt.Unix() {
		return true
	}
	if watermark.Total == 0 && meta.Total > 0 {
		return true
	}
	return false
}

func (s *Snapshotter) verifyContestFromMain(ctx context.Context, contestID string) (bool, error) {
	if s.mainSummary == nil || s.redis == nil {
		return true, nil
	}
	var lastMemberID string
	var expectedTotal int64
	for {
		ctxDB := withTimeout(ctx, s.dbTimeout)
		rows, err := s.mainSummary.ListByContestAfterMember(ctxDB.ctx, contestID, lastMemberID, s.recovery.RebuildBatchSize)
		ctxDB.cancel()
		if err != nil {
			return false, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			expectedTotal++
			summary, found, err := s.loadRedisSummary(ctx, contestID, row.MemberID)
			if err != nil {
				return false, err
			}
			if !found {
				return false, nil
			}
			if summary.ScoreTotal != row.ScoreTotal || summary.Penalty != row.PenaltyTotal || summary.ACCount != row.ACCount {
				return false, nil
			}
			if parseVersion(summary.Version) < row.Version {
				return false, nil
			}
			if s.recovery.VerifyStrict && summary.DetailJSON != row.DetailJSON {
				return false, nil
			}
		}
		lastMemberID = rows[len(rows)-1].MemberID
	}
	if !s.recovery.VerifyStrict {
		return true, nil
	}
	ctxCache := withTimeout(ctx, s.cacheTimeout)
	total, err := s.redis.ZcardCtx(ctxCache.ctx, repository.LeaderboardKey(contestID))
	ctxCache.cancel()
	if err != nil {
		return false, err
	}
	return int64(total) == expectedTotal, nil
}

func (s *Snapshotter) loadRedisSummary(ctx context.Context, contestID, memberID string) (pmodel.LeaderboardSummary, bool, error) {
	ctxCache := withTimeout(ctx, s.cacheTimeout)
	raw, err := s.redis.HgetCtx(ctxCache.ctx, repository.DetailKey(contestID, memberID), "summary")
	ctxCache.cancel()
	if err != nil {
		if err == red.Nil {
			return pmodel.LeaderboardSummary{}, false, nil
		}
		return pmodel.LeaderboardSummary{}, false, err
	}
	if raw == "" {
		return pmodel.LeaderboardSummary{}, false, nil
	}
	var summary pmodel.LeaderboardSummary
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return pmodel.LeaderboardSummary{}, false, err
	}
	return summary, true, nil
}

func (s *Snapshotter) rebuildContestFromMain(ctx context.Context, contestID string) error {
	if s.mainSummary == nil || s.leaderboard == nil {
		return fmt.Errorf("rebuild dependency is not configured")
	}
	var (
		lastMemberID string
		buffer       []pmodel.RankUpdateEvent
	)
	flush := func() error {
		if len(buffer) == 0 {
			return nil
		}
		if err := s.leaderboard.ApplyUpdates(ctx, buffer); err != nil {
			return err
		}
		buffer = buffer[:0]
		return nil
	}
	for {
		ctxDB := withTimeout(ctx, s.dbTimeout)
		rows, err := s.mainSummary.ListByContestAfterMember(ctxDB.ctx, contestID, lastMemberID, s.recovery.RebuildBatchSize)
		ctxDB.cancel()
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			version := row.Version
			if version <= 0 {
				version = 1
			}
			buffer = append(buffer, pmodel.RankUpdateEvent{
				ContestID:  contestID,
				MemberID:   row.MemberID,
				SortScore:  score.SortScore(row.ScoreTotal, row.PenaltyTotal),
				ScoreTotal: row.ScoreTotal,
				Penalty:    row.PenaltyTotal,
				ACCount:    row.ACCount,
				DetailJSON: row.DetailJSON,
				Version:    strconv.FormatInt(version, 10),
				ResultID:   0,
				UpdatedAt:  row.UpdatedAt.Unix(),
			})
			if len(buffer) >= s.recovery.RebuildBatchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
		lastMemberID = rows[len(rows)-1].MemberID
	}
	return flush()
}

type timeoutCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func withTimeout(ctx context.Context, timeout time.Duration) timeoutCtx {
	if timeout <= 0 {
		return timeoutCtx{ctx: ctx, cancel: func() {}}
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx{ctx: ctxTimeout, cancel: cancel}
}

func parseInt64(values []string, idx int) int64 {
	if idx < 0 || idx >= len(values) {
		return 0
	}
	if values[idx] == "" {
		return 0
	}
	val, err := strconv.ParseInt(values[idx], 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func parseVersion(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}
