package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"fuzoj/internal/common/mq"
	"fuzoj/internal/gateway/repository"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

type BanEvent struct {
	EventType string `json:"event_type"`
	UserID    int64  `json:"user_id"`
	EndTime   string `json:"end_time"`
}

// BanEventConsumer subscribes to user ban events and updates local cache.
type BanEventConsumer struct {
	mq        mq.MessageQueue
	banRepo   *repository.BanCacheRepository
	defaultTTL time.Duration
}

func NewBanEventConsumer(mqClient mq.MessageQueue, banRepo *repository.BanCacheRepository, defaultTTL time.Duration) *BanEventConsumer {
	return &BanEventConsumer{mq: mqClient, banRepo: banRepo, defaultTTL: defaultTTL}
}

func (c *BanEventConsumer) Subscribe(ctx context.Context, topic, group string) error {
	if c.mq == nil {
		return errors.New("message queue is nil")
	}
	if c.banRepo == nil {
		return errors.New("ban repository is nil")
	}
	opts := &mq.SubscribeOptions{ConsumerGroup: group, Concurrency: 2}
	if err := c.mq.SubscribeWithOptions(ctx, topic, c.handleMessage, opts); err != nil {
		return err
	}
	return c.mq.Start()
}

func (c *BanEventConsumer) handleMessage(ctx context.Context, message *mq.Message) error {
	if message == nil {
		return nil
	}
	var event BanEvent
	if err := json.Unmarshal(message.Body, &event); err != nil {
		logger.Warn(ctx, "parse ban event failed", zap.Error(err))
		return nil
	}
	if event.UserID == 0 {
		return nil
	}

	switch event.EventType {
	case "user.banned":
		ttl := c.defaultTTL
		if event.EndTime != "" {
			if parsed, err := parseEventTime(event.EndTime); err == nil {
				if until := time.Until(parsed); until > 0 {
					ttl = until
				}
			}
		}
		c.banRepo.MarkBanned(event.UserID, ttl)
	case "user.unbanned":
		c.banRepo.UnmarkBanned(event.UserID)
	}
	return nil
}

func parseEventTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("empty time")
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts, nil
	}
	return time.Parse(time.RFC3339, raw)
}
