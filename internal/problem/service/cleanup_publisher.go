package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/mq"
	"fuzoj/internal/problem/model"
)

// ProblemCleanupPublisher publishes async cleanup events.
type ProblemCleanupPublisher struct {
	queue     mq.MessageQueue
	topic     string
	bucket    string
	keyPrefix string
}

// NewProblemCleanupPublisher creates a new cleanup event publisher.
func NewProblemCleanupPublisher(queue mq.MessageQueue, topic, bucket, keyPrefix string) *ProblemCleanupPublisher {
	return &ProblemCleanupPublisher{
		queue:     queue,
		topic:     topic,
		bucket:    bucket,
		keyPrefix: keyPrefix,
	}
}

// PublishProblemDeleted publishes a cleanup event for the deleted problem.
func (p *ProblemCleanupPublisher) PublishProblemDeleted(ctx context.Context, problemID int64) error {
	if p == nil || p.queue == nil {
		return errors.New("cleanup publisher is nil")
	}
	if p.topic == "" {
		return errors.New("cleanup topic is empty")
	}
	if problemID <= 0 {
		return errors.New("problemID is required")
	}
	event := model.ProblemCleanupEvent{
		EventType:   model.ProblemCleanupEventDeleted,
		ProblemID:   problemID,
		Bucket:      p.bucket,
		Prefix:      problemObjectPrefix(p.keyPrefix, problemID),
		RequestedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal cleanup event failed: %w", err)
	}
	message := mq.NewMessage(payload)
	message.ID = fmt.Sprintf("problem-delete-%d-%d", problemID, time.Now().UnixNano())
	if err := p.queue.Publish(ctx, p.topic, message); err != nil {
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
