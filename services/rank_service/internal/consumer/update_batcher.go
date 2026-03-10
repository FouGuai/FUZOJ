package consumer

import (
	"context"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_service/internal/pmodel"
	"fuzoj/services/rank_service/internal/repository"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// UpdateBatcher batches leaderboard updates before persisting.
type UpdateBatcher struct {
	repo         repository.UpdateApplier
	pubsub       *red.Client
	size         int
	interval     time.Duration
	applyTimeout time.Duration
	ch           chan queuedRankUpdate
	stop         chan struct{}
}

type queuedRankUpdate struct {
	event      pmodel.RankUpdateEvent
	enqueuedAt time.Time
}

const (
	rankBatcherDiagnosticsInterval = 5 * time.Second
	rankQueueWarnThreshold         = time.Second
	rankApplyWarnThreshold         = 500 * time.Millisecond
)

func NewUpdateBatcher(repo repository.UpdateApplier, pubsub *red.Client, size int, interval, applyTimeout time.Duration) *UpdateBatcher {
	if size <= 0 {
		size = 200
	}
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	if applyTimeout <= 0 {
		applyTimeout = 5 * time.Second
	}
	return &UpdateBatcher{
		repo:         repo,
		pubsub:       pubsub,
		size:         size,
		interval:     interval,
		applyTimeout: applyTimeout,
		ch:           make(chan queuedRankUpdate, size*4),
		stop:         make(chan struct{}),
	}
}

func (b *UpdateBatcher) Start(ctx context.Context) {
	logger := logx.WithContext(ctx)
	logger.Info("rank update batcher started")
	ticker := time.NewTicker(b.interval)
	diagTicker := time.NewTicker(rankBatcherDiagnosticsInterval)
	defer ticker.Stop()
	defer diagTicker.Stop()
	buffer := make([]queuedRankUpdate, 0, b.size)
	var pending []queuedRankUpdate
	retryDelay := 100 * time.Millisecond
	nextRetry := time.Time{}
	flush := func() {
		if len(pending) == 0 && len(buffer) == 0 {
			return
		}
		if len(pending) > 0 && !nextRetry.IsZero() && time.Now().Before(nextRetry) {
			return
		}
		var batch []queuedRankUpdate
		if len(pending) > 0 {
			batch = pending
		} else {
			batch = make([]queuedRankUpdate, len(buffer))
			copy(batch, buffer)
			buffer = buffer[:0]
		}
		applyStart := time.Now()
		maxQueueWait, avgQueueWait := queueWaitStats(batch, applyStart)
		events := unwrapQueuedEvents(batch)
		applyCtx := ctx
		if b.applyTimeout > 0 {
			var cancel context.CancelFunc
			applyCtx, cancel = context.WithTimeout(ctx, b.applyTimeout)
			defer cancel()
		}
		if err := b.repo.ApplyUpdates(applyCtx, events); err != nil {
			logger.Errorf("apply rank updates failed: %v", err)
			pending = batch
			if retryDelay < 2*time.Second {
				retryDelay *= 2
			}
			nextRetry = time.Now().Add(retryDelay)
			return
		}
		pending = nil
		retryDelay = 100 * time.Millisecond
		nextRetry = time.Time{}
		applyCost := time.Since(applyStart)
		if maxQueueWait >= rankQueueWarnThreshold || applyCost >= rankApplyWarnThreshold {
			logger.Infof("rank batch flush stats size=%d queue_wait_max=%s queue_wait_avg=%s apply_cost=%s",
				len(events), maxQueueWait, avgQueueWait, applyCost)
		}
		b.publish(ctx, events)
	}

	for {
		select {
		case <-b.stop:
			for {
				select {
				case event := <-b.ch:
					buffer = append(buffer, event)
				default:
					flush()
					logger.Info("rank update batcher stopped")
					return
				}
			}
		case <-ticker.C:
			flush()
		case <-diagTicker.C:
			queued := len(b.ch)
			if queued == 0 && len(buffer) == 0 && len(pending) == 0 {
				continue
			}
			if queued >= cap(b.ch)*8/10 {
				logger.Errorf("rank batcher queue pressure high queued=%d capacity=%d buffer=%d pending=%d", queued, cap(b.ch), len(buffer), len(pending))
				continue
			}
			logger.Infof("rank batcher queue snapshot queued=%d capacity=%d buffer=%d pending=%d", queued, cap(b.ch), len(buffer), len(pending))
		case event := <-b.ch:
			buffer = append(buffer, event)
			if len(buffer) >= b.size {
				flush()
			}
		}
	}
}

func (b *UpdateBatcher) Stop() {
	close(b.stop)
}

func (b *UpdateBatcher) Add(ctx context.Context, event pmodel.RankUpdateEvent) error {
	item := queuedRankUpdate{
		event:      event,
		enqueuedAt: time.Now(),
	}
	select {
	case b.ch <- item:
		return nil
	case <-ctx.Done():
		return appErr.New(appErr.Timeout).WithMessage("rank update enqueue timeout")
	}
}

func (b *UpdateBatcher) publish(ctx context.Context, events []pmodel.RankUpdateEvent) {
	if b.pubsub == nil || len(events) == 0 {
		return
	}
	logger := logx.WithContext(ctx)
	seen := make(map[string]struct{}, len(events))
	for _, event := range events {
		if event.ContestID == "" {
			continue
		}
		if _, ok := seen[event.ContestID]; ok {
			continue
		}
		seen[event.ContestID] = struct{}{}
		channel := "contest:lb:pubsub:" + event.ContestID
		if err := b.pubsub.Publish(ctx, channel, event.Version).Err(); err != nil {
			logger.Errorf("publish rank update failed: %v", err)
		}
	}
}

func unwrapQueuedEvents(items []queuedRankUpdate) []pmodel.RankUpdateEvent {
	if len(items) == 0 {
		return nil
	}
	out := make([]pmodel.RankUpdateEvent, 0, len(items))
	for _, item := range items {
		out = append(out, item.event)
	}
	return out
}

func queueWaitStats(items []queuedRankUpdate, now time.Time) (time.Duration, time.Duration) {
	if len(items) == 0 {
		return 0, 0
	}
	var maxWait time.Duration
	var total time.Duration
	for _, item := range items {
		wait := now.Sub(item.enqueuedAt)
		if wait > maxWait {
			maxWait = wait
		}
		total += wait
	}
	return maxWait, total / time.Duration(len(items))
}
