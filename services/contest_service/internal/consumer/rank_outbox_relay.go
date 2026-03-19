package consumer

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"fuzoj/services/contest_service/internal/repository"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type RankOutboxRelayOptions struct {
	OwnerID             string
	ContestScanInterval time.Duration
	ContestScanBatch    int
	ClaimBatchSize      int
	LeaseDuration       time.Duration
	LeaseRenewInterval  time.Duration
	RetryBaseDelay      time.Duration
	RetryMaxDelay       time.Duration
	CleanupInterval     time.Duration
	SentRetention       time.Duration
	CleanupBatchSize    int
	RequeueBatchSize    int
	DBTimeout           time.Duration
	MQTimeout           time.Duration
}

// RankOutboxRelay publishes outbox events to Kafka.
type RankOutboxRelay struct {
	repo    *repository.RankOutboxRepository
	pusher  *kq.Pusher
	redis   *redis.Redis
	options RankOutboxRelayOptions
	stopCh  chan struct{}
	once    sync.Once
}

const (
	rankOutboxBacklogWarnThreshold = 2 * time.Second
	rankOutboxPublishWarnThreshold = 500 * time.Millisecond
	rankOutboxLockKeyPrefix        = "contest:rank:outbox:lock:"
)

func NewRankOutboxRelay(repo *repository.RankOutboxRepository, pusher *kq.Pusher, redisClient *redis.Redis, options RankOutboxRelayOptions) *RankOutboxRelay {
	if options.OwnerID == "" {
		host, _ := os.Hostname()
		options.OwnerID = fmt.Sprintf("%s-%d", host, time.Now().UnixNano())
	}
	if options.ContestScanInterval <= 0 {
		options.ContestScanInterval = time.Second
	}
	if options.ContestScanBatch <= 0 {
		options.ContestScanBatch = 32
	}
	if options.ClaimBatchSize <= 0 {
		options.ClaimBatchSize = 200
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 3 * time.Second
	}
	if options.LeaseRenewInterval <= 0 {
		options.LeaseRenewInterval = time.Second
	}
	if options.RetryBaseDelay <= 0 {
		options.RetryBaseDelay = time.Second
	}
	if options.RetryMaxDelay <= 0 {
		options.RetryMaxDelay = 30 * time.Second
	}
	if options.CleanupInterval <= 0 {
		options.CleanupInterval = time.Minute
	}
	if options.SentRetention <= 0 {
		options.SentRetention = 15 * time.Minute
	}
	if options.CleanupBatchSize <= 0 {
		options.CleanupBatchSize = 200
	}
	if options.RequeueBatchSize <= 0 {
		options.RequeueBatchSize = 200
	}
	return &RankOutboxRelay{
		repo:    repo,
		pusher:  pusher,
		redis:   redisClient,
		options: options,
		stopCh:  make(chan struct{}),
	}
}

func (r *RankOutboxRelay) Start() {
	if r == nil {
		return
	}
	logx.Infof("rank outbox relay started, owner=%s", r.options.OwnerID)
	go r.run(context.Background())
}

func (r *RankOutboxRelay) run(ctx context.Context) {
	logger := logx.WithContext(ctx)
	lastCleanup := time.Now()
	for {
		select {
		case <-r.stopCh:
			logger.Infof("rank outbox relay stopped, owner=%s", r.options.OwnerID)
			return
		default:
		}

		now := time.Now()
		r.requeueExpired(ctx, now)
		if now.Sub(lastCleanup) >= r.options.CleanupInterval {
			r.cleanupSent(ctx, now)
			lastCleanup = now
		}

		contests, err := r.listPendingContests(ctx, now)
		if err != nil {
			logger.Errorf("list pending contests failed: %v", err)
			time.Sleep(r.options.ContestScanInterval)
			continue
		}
		if len(contests) == 0 {
			time.Sleep(r.options.ContestScanInterval)
			continue
		}

		processedAny := false
		for _, contestID := range contests {
			lock, locked, err := r.acquireLease(ctx, contestID)
			if err != nil {
				logger.Errorf("acquire contest lease failed, contest=%s err=%v", contestID, err)
				continue
			}
			if !locked {
				continue
			}
			processed, err := r.processContest(ctx, contestID, lock)
			releaseErr := r.releaseLease(ctx, lock)
			if releaseErr != nil {
				logger.Errorf("release contest lease failed, contest=%s err=%v", contestID, releaseErr)
			}
			if err != nil {
				logger.Errorf("process contest outbox failed, contest=%s err=%v", contestID, err)
			}
			if processed {
				processedAny = true
			}
		}
		if !processedAny {
			time.Sleep(r.options.ContestScanInterval)
		}
	}
}

func (r *RankOutboxRelay) Stop() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		close(r.stopCh)
	})
}

func (r *RankOutboxRelay) processContest(ctx context.Context, contestID string, lock *redis.RedisLock) (bool, error) {
	logger := logx.WithContext(ctx)
	processedAny := false
	lastRenew := time.Now()

	for {
		if time.Since(lastRenew) >= r.options.LeaseRenewInterval {
			if err := r.renewLease(ctx, contestID, lock); err != nil {
				return processedAny, err
			}
			lastRenew = time.Now()
		}

		events, err := r.claimByContest(ctx, contestID)
		if err != nil {
			return processedAny, err
		}
		if len(events) == 0 {
			return processedAny, nil
		}

		processedAny = true
		batchStart := time.Now()
		sentIDs := make([]int64, 0, len(events))
		failedIDs := make(map[int][]int64)
		sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
		var oldestCreatedAt time.Time
		for _, event := range events {
			if oldestCreatedAt.IsZero() || event.CreatedAt.Before(oldestCreatedAt) {
				oldestCreatedAt = event.CreatedAt
			}
			if err := event.ValidateForClaim(contestID, r.options.OwnerID); err != nil {
				logger.Errorf("invalid claimed event, contest=%s id=%d err=%v", contestID, event.ID, err)
				failedIDs[event.RetryCount] = append(failedIDs[event.RetryCount], event.ID)
				continue
			}
			if event.Payload == "" {
				sentIDs = append(sentIDs, event.ID)
				continue
			}
			ctxMQ := withTimeout(ctx, r.options.MQTimeout)
			err := r.pusher.PushWithKey(ctxMQ.ctx, event.KafkaKey, event.Payload)
			ctxMQ.cancel()
			if err != nil {
				logger.Errorf("publish rank update failed, contest=%s id=%d retry=%d err=%v", contestID, event.ID, event.RetryCount, err)
				failedIDs[event.RetryCount] = append(failedIDs[event.RetryCount], event.ID)
				continue
			}
			sentIDs = append(sentIDs, event.ID)
		}

		if len(sentIDs) > 0 {
			ctxDB := withTimeout(ctx, r.options.DBTimeout)
			err := r.repo.MarkSentByOwner(ctxDB.ctx, r.options.OwnerID, sentIDs)
			ctxDB.cancel()
			if err != nil {
				logger.Errorf("mark sent failed, contest=%s count=%d err=%v", contestID, len(sentIDs), err)
			}
		}

		if len(failedIDs) > 0 {
			levels := sortedRetryCounts(failedIDs)
			for _, retryCount := range levels {
				nextRetry := time.Now().Add(backoffDuration(retryCount, r.options.RetryBaseDelay, r.options.RetryMaxDelay))
				ctxDB := withTimeout(ctx, r.options.DBTimeout)
				err := r.repo.MarkFailedWithRetry(ctxDB.ctx, r.options.OwnerID, retryCount, failedIDs[retryCount], nextRetry)
				ctxDB.cancel()
				if err != nil {
					logger.Errorf("mark failed batch failed, contest=%s retry=%d count=%d err=%v", contestID, retryCount, len(failedIDs[retryCount]), err)
				}
			}
		}
		processCost := time.Since(batchStart)
		backlogAge := time.Duration(0)
		if !oldestCreatedAt.IsZero() {
			backlogAge = time.Since(oldestCreatedAt)
		}
		if backlogAge >= rankOutboxBacklogWarnThreshold || processCost >= rankOutboxPublishWarnThreshold || len(failedIDs) > 0 {
			logger.Infof("rank outbox relay stats contest=%s claimed=%d sent=%d failed=%d backlog_age=%s process_cost=%s",
				contestID, len(events), len(sentIDs), countFailed(failedIDs), backlogAge, processCost)
		}

		if len(events) < r.options.ClaimBatchSize {
			return processedAny, nil
		}
	}
}

func (r *RankOutboxRelay) requeueExpired(ctx context.Context, now time.Time) {
	logger := logx.WithContext(ctx)
	ctxDB := withTimeout(ctx, r.options.DBTimeout)
	affected, err := r.repo.RequeueExpiredProcessing(ctxDB.ctx, now, r.options.RequeueBatchSize)
	ctxDB.cancel()
	if err != nil {
		logger.Errorf("requeue expired processing failed: %v", err)
		return
	}
	if affected > 0 {
		logger.Infof("requeued expired processing events: %d", affected)
	}
}

func (r *RankOutboxRelay) cleanupSent(ctx context.Context, now time.Time) {
	logger := logx.WithContext(ctx)
	ctxDB := withTimeout(ctx, r.options.DBTimeout)
	affected, err := r.repo.DeleteSentBefore(ctxDB.ctx, now.Add(-r.options.SentRetention), r.options.CleanupBatchSize)
	ctxDB.cancel()
	if err != nil {
		logger.Errorf("cleanup sent outbox failed: %v", err)
		return
	}
	if affected > 0 {
		logger.Infof("cleaned sent outbox rows: %d", affected)
	}
}

func (r *RankOutboxRelay) listPendingContests(ctx context.Context, now time.Time) ([]string, error) {
	ctxDB := withTimeout(ctx, r.options.DBTimeout)
	defer ctxDB.cancel()
	return r.repo.ListPendingContests(ctxDB.ctx, now, r.options.ContestScanBatch)
}

func (r *RankOutboxRelay) acquireLease(ctx context.Context, contestID string) (*redis.RedisLock, bool, error) {
	if r.redis == nil {
		return nil, false, fmt.Errorf("redis is not configured")
	}
	lock := redis.NewRedisLock(r.redis, buildRankOutboxLockKey(contestID))
	lock.SetExpire(lockExpireSeconds(r.options.LeaseDuration))
	ctxLock := withTimeout(ctx, r.options.DBTimeout)
	ok, err := lock.AcquireCtx(ctxLock.ctx)
	ctxLock.cancel()
	if err != nil {
		return nil, false, err
	}
	return lock, ok, nil
}

func (r *RankOutboxRelay) renewLease(ctx context.Context, contestID string, lock *redis.RedisLock) error {
	if lock == nil {
		return fmt.Errorf("lock is required for contest %s", contestID)
	}
	ctxLock := withTimeout(ctx, r.options.DBTimeout)
	ok, err := lock.AcquireCtx(ctxLock.ctx)
	ctxLock.cancel()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("renew contest lease failed, contest=%s", contestID)
	}
	return nil
}

func (r *RankOutboxRelay) releaseLease(ctx context.Context, lock *redis.RedisLock) error {
	if lock == nil {
		return nil
	}
	ctxLock := withTimeout(ctx, r.options.DBTimeout)
	_, err := lock.ReleaseCtx(ctxLock.ctx)
	ctxLock.cancel()
	return err
}

func (r *RankOutboxRelay) claimByContest(ctx context.Context, contestID string) ([]repository.RankOutboxEvent, error) {
	ctxDB := withTimeout(ctx, r.options.DBTimeout)
	defer ctxDB.cancel()
	return r.repo.ClaimByContest(ctxDB.ctx, contestID, r.options.OwnerID, r.options.ClaimBatchSize, r.options.LeaseDuration)
}

func backoffDuration(retryCount int, base, max time.Duration) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	delay := base * time.Duration(retryCount+1)
	if delay > max {
		return max
	}
	return delay
}

func sortedRetryCounts(groups map[int][]int64) []int {
	levels := make([]int, 0, len(groups))
	for level := range groups {
		levels = append(levels, level)
	}
	sort.Ints(levels)
	return levels
}

func countFailed(groups map[int][]int64) int {
	if len(groups) == 0 {
		return 0
	}
	total := 0
	for _, ids := range groups {
		total += len(ids)
	}
	return total
}

func buildRankOutboxLockKey(contestID string) string {
	return rankOutboxLockKeyPrefix + contestID
}

func lockExpireSeconds(duration time.Duration) int {
	if duration <= 0 {
		return 1
	}
	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds <= 0 {
		return 1
	}
	return seconds
}
