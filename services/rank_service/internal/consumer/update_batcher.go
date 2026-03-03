package consumer

import (
	"context"
	"time"

	"fuzoj/services/rank_service/internal/pmodel"
	"fuzoj/services/rank_service/internal/repository"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// UpdateBatcher batches leaderboard updates before persisting.
type UpdateBatcher struct {
	repo     *repository.LeaderboardRepository
	pubsub   *red.Client
	size     int
	interval time.Duration
	ch       chan pmodel.RankUpdateEvent
	stop     chan struct{}
}

func NewUpdateBatcher(repo *repository.LeaderboardRepository, pubsub *red.Client, size int, interval time.Duration) *UpdateBatcher {
	if size <= 0 {
		size = 200
	}
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	return &UpdateBatcher{
		repo:     repo,
		pubsub:   pubsub,
		size:     size,
		interval: interval,
		ch:       make(chan pmodel.RankUpdateEvent, size*4),
		stop:     make(chan struct{}),
	}
}

func (b *UpdateBatcher) Start(ctx context.Context) {
	logger := logx.WithContext(ctx)
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	buffer := make([]pmodel.RankUpdateEvent, 0, b.size)
	flush := func() {
		if len(buffer) == 0 {
			return
		}
		batch := make([]pmodel.RankUpdateEvent, len(buffer))
		copy(batch, buffer)
		buffer = buffer[:0]
		if err := b.repo.ApplyUpdates(ctx, batch); err != nil {
			logger.Errorf("apply rank updates failed: %v", err)
			return
		}
		b.publish(ctx, batch)
	}

	for {
		select {
		case <-b.stop:
			flush()
			return
		case <-ticker.C:
			flush()
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

func (b *UpdateBatcher) Add(event pmodel.RankUpdateEvent) {
	select {
	case b.ch <- event:
	default:
		logx.Error("rank update batcher is full")
	}
}

func (b *UpdateBatcher) publish(ctx context.Context, events []pmodel.RankUpdateEvent) {
	if b.pubsub == nil || len(events) == 0 {
		return
	}
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
		_ = b.pubsub.Publish(ctx, channel, event.Version).Err()
	}
}
