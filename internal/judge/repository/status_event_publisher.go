package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"fuzoj/internal/common/mq"
	"fuzoj/internal/judge/model"
	appErr "fuzoj/pkg/errors"
)

// StatusEventPublisher publishes status events for async processing.
type StatusEventPublisher interface {
	PublishFinalStatus(ctx context.Context, status model.JudgeStatusResponse) error
}

// MQStatusEventPublisher publishes status events to a message queue.
type MQStatusEventPublisher struct {
	queue mq.MessageQueue
	topic string
}

// NewMQStatusEventPublisher creates a new MQ status event publisher.
func NewMQStatusEventPublisher(queue mq.MessageQueue, topic string) *MQStatusEventPublisher {
	return &MQStatusEventPublisher{queue: queue, topic: topic}
}

// PublishFinalStatus publishes a final status event.
func (p *MQStatusEventPublisher) PublishFinalStatus(ctx context.Context, status model.JudgeStatusResponse) error {
	if p == nil || p.queue == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("status publisher is not configured")
	}
	if p.topic == "" {
		return appErr.New(appErr.InvalidParams).WithMessage("status topic is required")
	}
	if status.SubmissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	event := model.StatusEvent{
		Type:      model.StatusEventFinal,
		Status:    status,
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal status event failed: %w", err)
	}
	message := mq.NewMessage(payload)
	message.ID = status.SubmissionID
	if err := p.queue.Publish(ctx, p.topic, message); err != nil {
		return appErr.Wrapf(err, appErr.ServiceUnavailable, "publish status event failed")
	}
	return nil
}
