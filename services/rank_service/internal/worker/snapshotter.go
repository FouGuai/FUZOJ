package worker

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"fuzoj/services/rank_service/internal/pmodel"
	"fuzoj/services/rank_service/internal/repository"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Snapshotter periodically persists rank snapshots.
type Snapshotter struct {
	repo           *repository.SnapshotRepository
	redis          *redis.Redis
	interval       time.Duration
	pageSize       int
	batchSize      int
	cacheTimeout   time.Duration
	dbTimeout      time.Duration
	recoverOnStart bool
	stopCh         chan struct{}
	running        int32
}

func NewSnapshotter(repo *repository.SnapshotRepository, redisClient *redis.Redis, interval time.Duration, pageSize, batchSize int, cacheTimeout, dbTimeout time.Duration, recoverOnStart bool) *Snapshotter {
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
		redis:          redisClient,
		interval:       interval,
		pageSize:       pageSize,
		batchSize:      batchSize,
		cacheTimeout:   cacheTimeout,
		dbTimeout:      dbTimeout,
		recoverOnStart: recoverOnStart,
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
		ctxCache := withTimeout(ctx, s.cacheTimeout)
		exists, err := s.redis.ExistsCtx(ctxCache.ctx, repository.MetaKey(meta.ContestID))
		ctxCache.cancel()
		if err != nil {
			logger.Errorf("check rank meta failed: %v", err)
			continue
		}
		if exists {
			continue
		}
		if err := s.restoreSnapshot(ctx, meta); err != nil {
			logger.Errorf("restore rank snapshot failed: %v", err)
		}
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
			contestID := strings.TrimPrefix(key, prefix)
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

	snapshotAt := time.Now()
	ctxDB := withTimeout(ctx, s.dbTimeout)
	snapshotID, err := s.repo.CreateSnapshotMeta(ctxDB.ctx, repository.SnapshotMeta{
		ContestID:    contestID,
		SnapshotAt:   snapshotAt,
		LastResultID: lastResultID,
		LastVersion:  lastVersion,
		Total:        total,
		Status:       "writing",
	})
	ctxDB.cancel()
	if err != nil {
		return err
	}

	var start int64 = 0
	for start < total {
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
				memberID, ok := pair.Member.(string)
				if !ok || memberID == "" {
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
			memberID, ok := pair.Member.(string)
			if !ok || memberID == "" {
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
		err = s.redis.PipelinedCtx(ctxCache.ctx, func(pipe redis.Pipeliner) error {
			for _, entry := range entries {
				pipe.ZAdd(ctxCache.ctx, repository.LeaderboardKey(meta.ContestID), red.Z{
					Score:  float64(entry.SortScore),
					Member: entry.MemberID,
				})
				pipe.HSet(ctxCache.ctx, repository.DetailKey(meta.ContestID, entry.MemberID), "summary", entry.SummaryJSON)
				if entry.Rank > lastRank {
					lastRank = entry.Rank
				}
			}
			return nil
		})
		ctxCache.cancel()
		if err != nil {
			return err
		}
	}

	ctxCache := withTimeout(ctx, s.cacheTimeout)
	_ = s.redis.PipelinedCtx(ctxCache.ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctxCache.ctx, repository.MetaKey(meta.ContestID), "result_id", strconv.FormatInt(meta.LastResultID, 10))
		if meta.LastVersion > 0 {
			pipe.HSet(ctxCache.ctx, repository.MetaKey(meta.ContestID), "version", strconv.FormatInt(meta.LastVersion, 10))
		}
		pipe.HSet(ctxCache.ctx, repository.MetaKey(meta.ContestID), "updated_at", strconv.FormatInt(meta.SnapshotAt.Unix(), 10))
		pipe.HSet(ctxCache.ctx, repository.MetaKey(meta.ContestID), "snapshot_at", strconv.FormatInt(meta.SnapshotAt.Unix(), 10))
		return nil
	})
	ctxCache.cancel()

	logger.Infof("rank snapshot restored contest_id=%s", meta.ContestID)
	return nil
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

