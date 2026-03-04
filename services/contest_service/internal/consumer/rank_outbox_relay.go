package consumer

import (
	"context"
	"time"

	"fuzoj/services/contest_service/internal/repository"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

// RankOutboxRelay publishes outbox events to Kafka.
type RankOutboxRelay struct {
	repo      *repository.RankOutboxRepository
	pusher    *kq.Pusher
	interval  time.Duration
	batchSize int
	stopCh    chan struct{}
}

func NewRankOutboxRelay(repo *repository.RankOutboxRepository, pusher *kq.Pusher, interval time.Duration, batchSize int) *RankOutboxRelay {
	if interval <= 0 {
		interval = time.Second
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return &RankOutboxRelay{
		repo:      repo,
		pusher:    pusher,
		interval:  interval,
		batchSize: batchSize,
		stopCh:    make(chan struct{}),
	}
}

func (r *RankOutboxRelay) Start() {
	if r == nil {
		return
	}
	ticker := time.NewTicker(r.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-r.stopCh:
				return
			case <-ticker.C:
				r.flush(context.Background())
			}
		}
	}()
}

func (r *RankOutboxRelay) Stop() {
	if r == nil {
		return
	}
	close(r.stopCh)
}

func (r *RankOutboxRelay) flush(ctx context.Context) {
	logger := logx.WithContext(ctx)
	if r.repo == nil || r.pusher == nil {
		logger.Error("rank outbox relay is not configured")
		return
	}
	events, err := r.repo.ListPending(ctx, r.batchSize)
	if err != nil || len(events) == 0 {
		return
	}
	sentIDs := make([]int64, 0, len(events))
	for _, event := range events {
		if event.Payload == "" {
			sentIDs = append(sentIDs, event.ID)
			continue
		}
		if err := r.pusher.PushWithKey(ctx, event.KafkaKey, event.Payload); err != nil {
			logger.Errorf("publish rank update failed: %v", err)
			nextRetry := time.Now().Add(time.Second * time.Duration(1+event.RetryCount))
			_ = r.repo.MarkFailed(ctx, event, nextRetry)
			continue
		}
		sentIDs = append(sentIDs, event.ID)
	}
	if len(sentIDs) > 0 {
		_ = r.repo.MarkSent(ctx, sentIDs)
	}
}
