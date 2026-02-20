package service

import (
	"context"
	"strconv"
	"time"

	"fuzoj/internal/common/mq"
	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

const poolRetryHeader = "x-pool-retry"

func (s *Service) acquireSlot(ctx context.Context, submissionID string) error {
	select {
	case s.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return appErr.New(appErr.JudgeQueueFull).WithMessage("worker pool is full")
	}
}

func (s *Service) releaseSlot() {
	select {
	case <-s.sem:
	default:
	}
}

func (s *Service) tryAcquireSlot() bool {
	select {
	case s.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Service) requeueForPoolFull(ctx context.Context, msg *mq.Message) error {
	if s.queue == nil || s.retryTopic == "" {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("retry queue is not configured")
	}
	return RequeueForPoolFull(ctx, s.queue, s.retryTopic, s.deadLetter, s.poolRetryMax, s.poolRetryBase, s.poolRetryMaxD, msg)
}

func ParsePoolRetryCount(headers map[string]string) int {
	if headers == nil {
		return 0
	}
	raw, ok := headers[poolRetryHeader]
	if !ok {
		return 0
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val < 0 {
		return 0
	}
	return val
}

func CloneMessageForRetry(msg *mq.Message, retryCount int) *mq.Message {
	if msg == nil {
		return mq.NewMessage(nil)
	}
	out := &mq.Message{
		Body:       msg.Body,
		Headers:    make(map[string]string, len(msg.Headers)+1),
		Timestamp:  time.Now(),
		Priority:   msg.Priority,
		RetryCount: 0,
		MaxRetries: msg.MaxRetries,
		Expiration: msg.Expiration,
	}
	for k, v := range msg.Headers {
		out.Headers[k] = v
	}
	out.Headers[poolRetryHeader] = strconv.Itoa(retryCount)
	return out
}

func ComputePoolBackoff(retryCount int, base, max time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	if retryCount <= 0 {
		if max > 0 && base > max {
			return max
		}
		return base
	}
	delay := base
	for i := 0; i < retryCount; i++ {
		if max > 0 && delay >= max {
			return max
		}
		if max > 0 && delay > max/2 {
			delay = max
			break
		}
		delay *= 2
	}
	if max > 0 && delay > max {
		return max
	}
	return delay
}

// RequeueForPoolFull republishes a message when worker pool is full.
func RequeueForPoolFull(ctx context.Context, queue mq.MessageQueue, retryTopic, deadLetter string, maxRetry int, baseDelay, maxDelay time.Duration, msg *mq.Message) error {
	if queue == nil || retryTopic == "" {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("retry queue is not configured")
	}
	if msg == nil {
		return appErr.New(appErr.InvalidParams).WithMessage("message is nil")
	}
	retryCount := ParsePoolRetryCount(msg.Headers)
	if maxRetry > 0 && retryCount >= maxRetry {
		if deadLetter == "" {
			logger.Warn(ctx, "worker pool retry exhausted without dead letter", zap.Int("retry_count", retryCount), zap.String("message_id", msg.ID))
			return appErr.New(appErr.JudgeQueueFull).WithMessage("worker pool is full")
		}
		dead := CloneMessageForRetry(msg, retryCount)
		logger.Warn(ctx, "worker pool retry exhausted, sending to dead letter", zap.Int("retry_count", retryCount), zap.String("message_id", msg.ID), zap.String("topic", deadLetter))
		return queue.Publish(ctx, deadLetter, dead)
	}
	delay := ComputePoolBackoff(retryCount, baseDelay, maxDelay)
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			logger.Warn(ctx, "worker pool retry canceled during backoff", zap.Int("retry_count", retryCount), zap.String("message_id", msg.ID), zap.Duration("delay", delay))
			return ctx.Err()
		case <-timer.C:
		}
	}
	logger.Info(ctx, "worker pool requeue", zap.Int("retry_count", retryCount+1), zap.String("message_id", msg.ID), zap.Duration("delay", delay), zap.String("topic", retryTopic))
	requeued := CloneMessageForRetry(msg, retryCount+1)
	return queue.Publish(ctx, retryTopic, requeued)
}
