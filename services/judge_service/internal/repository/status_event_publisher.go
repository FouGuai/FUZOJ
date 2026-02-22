package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"fuzoj/internal/common/mq"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/pmodel"

	"github.com/zeromicro/go-zero/core/logx"
)

// StatusEventPublisher publishes status events for async processing.
type StatusEventPublisher interface {
	PublishFinalStatus(ctx context.Context, status pmodel.JudgeStatusResponse) error
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
func (p *MQStatusEventPublisher) PublishFinalStatus(ctx context.Context, status pmodel.JudgeStatusResponse) error {
	logger := logx.WithContext(ctx)
	logger.Infof("publish final status event start submission_id=%s", status.SubmissionID)
	if p == nil || p.queue == nil {
		logger.Error("status publisher is not configured")
		return appErr.New(appErr.ServiceUnavailable).WithMessage("status publisher is not configured")
	}
	if p.topic == "" {
		logger.Error("status topic is required")
		return appErr.New(appErr.InvalidParams).WithMessage("status topic is required")
	}
	if status.SubmissionID == "" {
		logger.Error("submission_id is required")
		return appErr.ValidationError("submission_id", "required")
	}
	event := pmodel.StatusEvent{
		Type:      pmodel.StatusEventFinal,
		Status:    status,
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		logger.Errorf("marshal status event failed: %v", err)
		return fmt.Errorf("marshal status event failed: %w", err)
	}
	message := mq.NewMessage(payload)
	message.ID = status.SubmissionID
	if err := p.queue.Publish(ctx, p.topic, message); err != nil {
		logger.Errorf("publish status event failed: %v", err)
		return appErr.Wrapf(err, appErr.ServiceUnavailable, "publish status event failed")
	}
	return nil
}
