package consumer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"fuzoj/pkg/submit/statuscache"
	"fuzoj/services/submit_service/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// DispatchRecoveryOptions configures timeout recovery relay.
type DispatchRecoveryOptions struct {
	Enabled       bool
	OwnerID       string
	TimeoutAfter  time.Duration
	ScanInterval  time.Duration
	ClaimBatch    int
	WorkerCount   int
	LeaseDuration time.Duration
	RetryBase     time.Duration
	RetryMax      time.Duration
	DBTimeout     time.Duration
	MQTimeout     time.Duration
}

// MessagePusher defines minimal publish behavior for recovery dispatch.
type MessagePusher interface {
	PushWithKey(ctx context.Context, key, value string) error
}

// DispatchTopicResolver resolves the publish target for a record.
type DispatchTopicResolver interface {
	ResolveDispatchTarget(record repository.SubmissionDispatchRecord) (name string, pusher MessagePusher)
}

// FinalStatusChecker checks whether a submission is finalized.
type FinalStatusChecker interface {
	HasFinalStatus(ctx context.Context, submissionID string) (bool, error)
}

// DispatchRecoveryRelay requeues timed-out submissions.
type DispatchRecoveryRelay struct {
	repo    repository.SubmissionDispatchRepository
	checker FinalStatusChecker
	router  DispatchTopicResolver
	cache   *redis.Redis
	options DispatchRecoveryOptions
	stopCh  chan struct{}
	once    sync.Once
}

// NewDispatchRecoveryRelay creates a timeout recovery relay.
func NewDispatchRecoveryRelay(
	repo repository.SubmissionDispatchRepository,
	checker FinalStatusChecker,
	router DispatchTopicResolver,
	cacheClient *redis.Redis,
	options DispatchRecoveryOptions,
) *DispatchRecoveryRelay {
	if options.OwnerID == "" {
		host, _ := os.Hostname()
		options.OwnerID = fmt.Sprintf("%s-%d", host, time.Now().UnixNano())
	}
	if options.TimeoutAfter <= 0 {
		options.TimeoutAfter = 2 * time.Minute
	}
	if options.ScanInterval <= 0 {
		options.ScanInterval = 2 * time.Second
	}
	if options.ClaimBatch <= 0 {
		options.ClaimBatch = 200
	}
	if options.WorkerCount <= 0 {
		options.WorkerCount = 8
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 5 * time.Second
	}
	if options.RetryBase <= 0 {
		options.RetryBase = time.Second
	}
	if options.RetryMax <= 0 {
		options.RetryMax = 30 * time.Second
	}
	return &DispatchRecoveryRelay{
		repo:    repo,
		checker: checker,
		router:  router,
		cache:   cacheClient,
		options: options,
		stopCh:  make(chan struct{}),
	}
}

// Start launches relay loop.
func (r *DispatchRecoveryRelay) Start() {
	if r == nil || !r.options.Enabled {
		return
	}
	if r.repo == nil || r.router == nil {
		logx.Error("dispatch recovery relay is disabled due to missing repository or router")
		return
	}
	logx.Infof("dispatch recovery relay started owner=%s", r.options.OwnerID)
	go r.run(context.Background())
}

// Stop stops relay loop.
func (r *DispatchRecoveryRelay) Stop() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		close(r.stopCh)
	})
}

func (r *DispatchRecoveryRelay) run(ctx context.Context) {
	logger := logx.WithContext(ctx)
	ticker := time.NewTicker(r.options.ScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			logger.Infof("dispatch recovery relay stopped owner=%s", r.options.OwnerID)
			return
		case <-ticker.C:
			r.recoverOnce(ctx)
		}
	}
}

func (r *DispatchRecoveryRelay) recoverOnce(ctx context.Context) {
	logger := logx.WithContext(ctx)
	now := time.Now()
	ctxDB := withTimeout(ctx, r.options.DBTimeout)
	affected, err := r.repo.RequeueExpiredProcessing(ctxDB.ctx, now, r.options.ClaimBatch)
	ctxDB.cancel()
	if err != nil {
		logger.Errorf("requeue expired processing failed: %v", err)
		return
	}
	if affected > 0 {
		logger.Infof("requeue expired processing rows=%d", affected)
	}

	ctxDB = withTimeout(ctx, r.options.DBTimeout)
	items, err := r.repo.ClaimDue(ctxDB.ctx, now, r.options.OwnerID, r.options.LeaseDuration, r.options.ClaimBatch)
	ctxDB.cancel()
	if err != nil {
		logger.Errorf("claim due submissions failed: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}
	r.processBatch(ctx, items)
}

func (r *DispatchRecoveryRelay) processBatch(ctx context.Context, items []repository.SubmissionDispatchRecord) {
	workers := r.options.WorkerCount
	if workers > len(items) {
		workers = len(items)
	}
	if workers <= 0 {
		return
	}

	jobs := make(chan repository.SubmissionDispatchRecord, len(items))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				r.processRecord(ctx, item)
			}
		}()
	}
	for _, item := range items {
		jobs <- item
	}
	close(jobs)
	wg.Wait()
}

func (r *DispatchRecoveryRelay) processRecord(ctx context.Context, item repository.SubmissionDispatchRecord) {
	logger := logx.WithContext(ctx)
	if strings.TrimSpace(item.SubmissionID) == "" {
		return
	}
	if r.checker != nil {
		ctxDB := withTimeout(ctx, r.options.DBTimeout)
		done, err := r.checker.HasFinalStatus(ctxDB.ctx, item.SubmissionID)
		ctxDB.cancel()
		if err != nil {
			logger.Errorf("check final status failed submission_id=%s err=%v", item.SubmissionID, err)
			_ = r.scheduleRetry(ctx, item, item.RetryCount+1)
			return
		}
		if done {
			ctxDone := withTimeout(ctx, r.options.DBTimeout)
			err := r.repo.MarkDone(ctxDone.ctx, nil, item.SubmissionID)
			ctxDone.cancel()
			if err != nil {
				logger.Errorf("mark dispatch done failed submission_id=%s err=%v", item.SubmissionID, err)
			}
			return
		}
	}

	name, pusher := r.router.ResolveDispatchTarget(item)
	if pusher == nil {
		logger.Errorf("dispatch target is not configured submission_id=%s target=%s", item.SubmissionID, name)
		_ = r.scheduleRetry(ctx, item, item.RetryCount+1)
		return
	}
	ctxMQ := withTimeout(ctx, r.options.MQTimeout)
	if err := r.deleteSubmissionStatusCache(ctxMQ.ctx, item.SubmissionID); err != nil {
		logger.Errorf("delete status cache before republish failed submission_id=%s err=%v", item.SubmissionID, err)
	}
	err := pusher.PushWithKey(ctxMQ.ctx, item.SubmissionID, item.Payload)
	ctxMQ.cancel()
	if err != nil {
		logger.Errorf("dispatch retry publish failed submission_id=%s target=%s err=%v", item.SubmissionID, name, err)
		_ = r.scheduleRetry(ctx, item, item.RetryCount+1)
		return
	}

	nextRetryAt := time.Now().Add(r.options.TimeoutAfter)
	ctxDB := withTimeout(ctx, r.options.DBTimeout)
	err = r.repo.MarkPublished(ctxDB.ctx, item.ID, r.options.OwnerID, nextRetryAt)
	ctxDB.cancel()
	if err != nil {
		logger.Errorf("mark dispatch published failed submission_id=%s err=%v", item.SubmissionID, err)
		return
	}
	logger.Infof("dispatch recovery republished submission_id=%s target=%s next_retry_at=%s", item.SubmissionID, name, nextRetryAt.Format(time.RFC3339))
}

func (r *DispatchRecoveryRelay) scheduleRetry(ctx context.Context, item repository.SubmissionDispatchRecord, retryCount int) error {
	delay := backoffDuration(retryCount, r.options.RetryBase, r.options.RetryMax)
	nextRetryAt := time.Now().Add(delay)
	ctxDB := withTimeout(ctx, r.options.DBTimeout)
	err := r.repo.MarkRetry(ctxDB.ctx, item.ID, r.options.OwnerID, retryCount, nextRetryAt)
	ctxDB.cancel()
	return err
}

func backoffDuration(retryCount int, base, max time.Duration) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	if retryCount <= 0 {
		return base
	}
	delay := base
	for i := 0; i < retryCount; i++ {
		if delay >= max {
			return max
		}
		delay *= 2
	}
	if delay > max {
		return max
	}
	return delay
}

func (r *DispatchRecoveryRelay) deleteSubmissionStatusCache(ctx context.Context, submissionID string) error {
	if r == nil || r.cache == nil || strings.TrimSpace(submissionID) == "" {
		return nil
	}
	_, err := r.cache.DelCtx(ctx, statuscache.PrimaryKey(submissionID), statuscache.LegacyKey(submissionID))
	return err
}
