package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/services/problem_service/internal/domain"

	"github.com/zeromicro/go-queue/kq"
)

// ProblemCleanupPublisher publishes async cleanup events.
type ProblemCleanupPublisher struct {
	pusher    *kq.Pusher
	topic     string
	bucket    string
	keyPrefix string
}

// NewProblemCleanupPublisher creates a new cleanup event publisher.
func NewProblemCleanupPublisher(brokers []string, topic, bucket, keyPrefix string) *ProblemCleanupPublisher {
	pusher := kq.NewPusher(brokers, topic, kq.WithSyncPush())
	return &ProblemCleanupPublisher{
		pusher:    pusher,
		topic:     topic,
		bucket:    bucket,
		keyPrefix: keyPrefix,
	}
}

// Close releases the underlying pusher resources.
func (p *ProblemCleanupPublisher) Close() error {
	if p == nil || p.pusher == nil {
		return nil
	}
	return p.pusher.Close()
}

// PublishProblemDeleted publishes a cleanup event for the deleted problem.
func (p *ProblemCleanupPublisher) PublishProblemDeleted(ctx context.Context, problemID int64) error {
	if p == nil || p.pusher == nil {
		return errors.New("cleanup publisher is nil")
	}
	if p.topic == "" {
		return errors.New("cleanup topic is empty")
	}
	if problemID <= 0 {
		return errors.New("problemID is required")
	}
	event := domain.ProblemCleanupEvent{
		EventType:   domain.ProblemCleanupEventDeleted,
		ProblemID:   problemID,
		Bucket:      p.bucket,
		Prefix:      problemObjectPrefix(p.keyPrefix, problemID),
		RequestedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal cleanup event failed: %w", err)
	}
	key := fmt.Sprintf("problem-delete-%d-%d", problemID, time.Now().UnixNano())
	if err := p.pusher.PushWithKey(ctx, key, string(payload)); err != nil {
		return fmt.Errorf("publish cleanup event failed: %w", err)
	}
	return nil
}

func problemObjectPrefix(keyPrefix string, problemID int64) string {
	if keyPrefix == "" {
		keyPrefix = "problems"
	}
	return fmt.Sprintf("%s/%d/", keyPrefix, problemID)
}
