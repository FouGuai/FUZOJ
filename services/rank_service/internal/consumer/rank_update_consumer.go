package consumer

import (
	"context"
	"encoding/json"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_service/internal/pmodel"
)

// RankUpdateConsumer consumes pre-computed leaderboard updates.
type RankUpdateConsumer struct {
	batcher *UpdateBatcher
	timeout time.Duration
}

func NewRankUpdateConsumer(batcher *UpdateBatcher, timeout time.Duration) *RankUpdateConsumer {
	return &RankUpdateConsumer{batcher: batcher, timeout: timeout}
}

func (c *RankUpdateConsumer) Consume(ctx context.Context, key, value string) error {
	if c == nil || c.batcher == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("rank batcher is not configured")
	}
	if value == "" {
		return nil
	}
	ctxMQ := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctxMQ, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	_ = ctxMQ
	var event pmodel.RankUpdateEvent
	if err := json.Unmarshal([]byte(value), &event); err != nil {
		return appErr.Wrapf(err, appErr.InvalidParams, "decode rank update failed")
	}
	if event.ContestID == "" || event.MemberID == "" {
		return appErr.ValidationError("contest_id", "required")
	}
	c.batcher.Add(event)
	return nil
}
