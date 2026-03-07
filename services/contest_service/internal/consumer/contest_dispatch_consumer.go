package consumer

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"fuzoj/pkg/contest/eligibility"
	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statuswriter"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	contestDispatchIdemKeyPrefix = "contest:dispatch:"
)

type DispatchOptions struct {
	Topic           string
	IdempotencyTTL  time.Duration
	MessageTTL      time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	DeadLetterTopic string
}

type TimeoutConfig struct {
	MQ    time.Duration
	Cache time.Duration
}

// ContestDispatchConsumer validates contest submissions and forwards to judge queue.
type ContestDispatchConsumer struct {
	eligibilityService *eligibility.Service
	statusWriter       *statuswriter.FinalStatusWriter
	redis              *redis.Redis
	judgePusher        MessagePusher
	deadLetterPusher   *kq.Pusher
	opts               DispatchOptions
	timeouts           TimeoutConfig
}

// MessagePusher defines minimal pusher interface for forwarding messages.
type MessagePusher interface {
	PushWithKey(ctx context.Context, key, value string) error
}

// NewContestDispatchConsumer creates a contest dispatch consumer.
func NewContestDispatchConsumer(
	eligibilityService *eligibility.Service,
	statusWriter *statuswriter.FinalStatusWriter,
	redisClient *redis.Redis,
	judgePusher MessagePusher,
	opts DispatchOptions,
	timeouts TimeoutConfig,
) *ContestDispatchConsumer {
	if opts.IdempotencyTTL <= 0 {
		opts.IdempotencyTTL = 30 * time.Minute
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 3
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = time.Second
	}
	return &ContestDispatchConsumer{
		eligibilityService: eligibilityService,
		statusWriter:       statusWriter,
		redis:              redisClient,
		judgePusher:        judgePusher,
		opts:               opts,
		timeouts:           timeouts,
	}
}

// SetDeadLetterPusher configures the dead-letter topic pusher.
func (c *ContestDispatchConsumer) SetDeadLetterPusher(pusher *kq.Pusher) {
	c.deadLetterPusher = pusher
}

// Consume handles contest dispatch messages.
func (c *ContestDispatchConsumer) Consume(ctx context.Context, key, value string) error {
	if value == "" {
		return nil
	}
	for attempt := 0; attempt <= c.opts.MaxRetries; attempt++ {
		if err := c.handle(ctx, key, value); err == nil {
			return nil
		} else if attempt >= c.opts.MaxRetries {
			if c.opts.DeadLetterTopic != "" && c.deadLetterPusher != nil {
				_ = c.deadLetterPusher.PushWithKey(ctx, key, value)
			}
			logx.WithContext(ctx).Errorf("contest dispatch failed after retries: %v", err)
			return nil
		}
		time.Sleep(c.opts.RetryDelay)
	}
	return nil
}

func (c *ContestDispatchConsumer) handle(ctx context.Context, key, value string) error {
	logger := logx.WithContext(ctx)
	logger.Infof("contest dispatch consume start key=%s", key)
	if c == nil || c.eligibilityService == nil || c.statusWriter == nil {
		logger.Error("contest dispatch consumer is not configured")
		return appErr.New(appErr.ServiceUnavailable).WithMessage("contest dispatch is not configured")
	}

	var payload contestDispatchMessage
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		logger.Errorf("decode contest dispatch message failed: %v", err)
		return nil
	}
	if payload.SubmissionID == "" || payload.ContestID == "" || strings.TrimSpace(payload.UserID) == "" || payload.ProblemID <= 0 {
		logger.Error("contest dispatch message missing required fields")
		return nil
	}
	userID, err := strconv.ParseInt(payload.UserID, 10, 64)
	if err != nil || userID <= 0 {
		logger.Error("contest dispatch message user_id is invalid")
		return nil
	}
	if c.opts.MessageTTL > 0 && payload.CreatedAt > 0 {
		createdAt := time.Unix(payload.CreatedAt, 0)
		if time.Since(createdAt) > c.opts.MessageTTL {
			logger.Infof("contest dispatch message expired submission_id=%s", payload.SubmissionID)
			return nil
		}
	}

	var idemKey string
	var idemSet bool
	if c.redis != nil {
		idemKey = contestDispatchIdemKeyPrefix + payload.SubmissionID
		ok, err := c.redis.SetnxExCtx(ctx, idemKey, "processing", ttlSeconds(c.opts.IdempotencyTTL))
		if err != nil {
			logger.Errorf("contest dispatch idempotency failed: %v", err)
			return appErr.Wrapf(err, appErr.CacheError, "contest dispatch idempotency failed")
		}
		if !ok {
			logger.Infof("contest dispatch message skipped due to idempotency submission_id=%s", payload.SubmissionID)
			return nil
		}
		idemSet = true
	}

	ctxMQ := withTimeout(ctx, c.timeouts.MQ)
	defer ctxMQ.cancel()
	result, err := c.eligibilityService.Check(ctxMQ.ctx, eligibility.Request{
		ContestID: payload.ContestID,
		UserID:    userID,
		ProblemID: payload.ProblemID,
		Now:       time.Now(),
	})
	if err != nil {
		logger.Errorf("contest eligibility check failed: %v", err)
		c.clearIdempotency(ctx, idemKey, idemSet)
		return err
	}
	if !result.OK {
		logger.Infof("contest eligibility rejected submission_id=%s code=%d", payload.SubmissionID, result.ErrorCode)
		status := statuswriter.StatusPayload{
			SubmissionID: payload.SubmissionID,
			Status:       "Failed",
			ErrorCode:    int(result.ErrorCode),
			ErrorMessage: result.Message,
			Timestamps: statuswriter.Timestamps{
				ReceivedAt: payload.CreatedAt,
				FinishedAt: time.Now().Unix(),
			},
			Progress: statuswriter.Progress{
				TotalTests: 0,
				DoneTests:  0,
			},
		}
		if payload.CreatedAt == 0 {
			status.Timestamps.ReceivedAt = time.Now().Unix()
		}
		if err := c.statusWriter.WriteFinalStatus(ctxMQ.ctx, status); err != nil {
			logger.Errorf("write final status failed: %v", err)
			c.clearIdempotency(ctx, idemKey, idemSet)
			return err
		}
		return nil
	}

	if c.judgePusher == nil {
		logger.Error("judge topic is not configured")
		c.clearIdempotency(ctx, idemKey, idemSet)
		return appErr.New(appErr.ServiceUnavailable).WithMessage("judge topic is not configured")
	}
	if err := c.judgePusher.PushWithKey(ctxMQ.ctx, payload.SubmissionID, value); err != nil {
		logger.Errorf("publish judge message failed: %v submission_id=%s", err, payload.SubmissionID)
		c.clearIdempotency(ctx, idemKey, idemSet)
		return err
	}
	logger.Infof("contest dispatch forwarded to judge submission_id=%s", payload.SubmissionID)
	return nil
}

type contestDispatchMessage struct {
	SubmissionID string `json:"submission_id"`
	ProblemID    int64  `json:"problem_id"`
	ContestID    string `json:"contest_id"`
	UserID       string `json:"user_id"`
	CreatedAt    int64  `json:"created_at"`
}

type timeoutCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func withTimeout(ctx context.Context, timeout time.Duration) timeoutCtx {
	if timeout <= 0 {
		return timeoutCtx{ctx: ctx, cancel: func() {}}
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx{ctx: ctxTimeout, cancel: cancel}
}

func ttlSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func (c *ContestDispatchConsumer) clearIdempotency(ctx context.Context, key string, enabled bool) {
	if !enabled || key == "" || c.redis == nil {
		return
	}
	if _, err := c.redis.DelCtx(ctx, key); err != nil {
		logx.WithContext(ctx).Errorf("clear contest dispatch idempotency failed: %v", err)
	}
}
