package judge_app

import (
	"context"
	"encoding/json"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"
	"fuzoj/services/judge_service/internal/pmodel"

	"go.uber.org/zap"
)

// MessagePusher defines the push method needed for retry and dead letter.
type MessagePusher interface {
	PushWithKey(ctx context.Context, key, value string) error
}

func (s *JudgeApp) acquireSlot(ctx context.Context, submissionID string) error {
	select {
	case s.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return appErr.New(appErr.JudgeQueueFull).WithMessage("worker pool is full")
	}
}

func (s *JudgeApp) releaseSlot() {
	select {
	case <-s.sem:
	default:
	}
}

func (s *JudgeApp) tryAcquireSlot() bool {
	select {
	case s.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *JudgeApp) requeueForPoolFull(ctx context.Context, payload pmodel.JudgeMessage) error {
	if s.retryPusher == nil || s.retryTopic == "" {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("retry queue is not configured")
	}
	return RequeueForPoolFull(ctx, s.retryPusher, s.deadPusher, s.retryTopic, s.deadLetter, s.poolRetryMax, s.poolRetryBase, s.poolRetryMaxD, payload)
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
func RequeueForPoolFull(ctx context.Context, retryPusher, deadPusher MessagePusher, retryTopic, deadLetter string, maxRetry int, baseDelay, maxDelay time.Duration, payload pmodel.JudgeMessage) error {
	if retryPusher == nil || retryTopic == "" {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("retry queue is not configured")
	}
	retryCount := payload.PoolRetry
	if retryCount < 0 {
		retryCount = 0
	}
	if maxRetry > 0 && retryCount >= maxRetry {
		if deadLetter == "" || deadPusher == nil {
			logger.Warn(ctx, "worker pool retry exhausted without dead letter", zap.Int("retry_count", retryCount), zap.String("message_id", payload.SubmissionID))
			return appErr.New(appErr.JudgeQueueFull).WithMessage("worker pool is full")
		}
		payload.PoolRetry = retryCount
		raw, err := json.Marshal(payload)
		if err != nil {
			return appErr.Wrapf(err, appErr.InvalidParams, "encode retry message failed")
		}
		logger.Warn(ctx, "worker pool retry exhausted, sending to dead letter", zap.Int("retry_count", retryCount), zap.String("message_id", payload.SubmissionID), zap.String("topic", deadLetter))
		return deadPusher.PushWithKey(ctx, payload.SubmissionID, string(raw))
	}
	delay := ComputePoolBackoff(retryCount, baseDelay, maxDelay)
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			logger.Warn(ctx, "worker pool retry canceled during backoff", zap.Int("retry_count", retryCount), zap.String("message_id", payload.SubmissionID), zap.Duration("delay", delay))
			return ctx.Err()
		case <-timer.C:
		}
	}
	logger.Info(ctx, "worker pool requeue", zap.Int("retry_count", retryCount+1), zap.String("message_id", payload.SubmissionID), zap.Duration("delay", delay), zap.String("topic", retryTopic))
	payload.PoolRetry = retryCount + 1
	if payload.CreatedAt == 0 {
		payload.CreatedAt = time.Now().Unix()
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return appErr.Wrapf(err, appErr.InvalidParams, "encode retry message failed")
	}
	return retryPusher.PushWithKey(ctx, payload.SubmissionID, string(raw))
}
