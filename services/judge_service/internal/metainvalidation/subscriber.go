package metainvalidation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"fuzoj/pkg/problem/metapubsub"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type problemMetaInvalidator interface {
	InvalidateProblemMeta(problemID int64)
}

// Subscriber listens for latest-problem-meta invalidation events.
type Subscriber struct {
	client      *red.Client
	invalidator problemMetaInvalidator
	cancel      context.CancelFunc
	done        chan struct{}
}

func NewSubscriber(client *red.Client, invalidator problemMetaInvalidator) *Subscriber {
	return &Subscriber{
		client:      client,
		invalidator: invalidator,
		done:        make(chan struct{}),
	}
}

func (s *Subscriber) Start(ctx context.Context) {
	if s == nil || s.client == nil || s.invalidator == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go s.run(runCtx)
}

func (s *Subscriber) Stop() {
	if s == nil {
		return
	}
	if s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
}

func (s *Subscriber) run(ctx context.Context) {
	defer close(s.done)
	logger := logx.WithContext(ctx)
	pubsub := s.client.Subscribe(ctx, metapubsub.Channel())
	defer func() { _ = pubsub.Close() }()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, red.ErrClosed) {
				return
			}
			logger.Errorf("receive problem meta invalidation failed: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		var event metapubsub.Event
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			logger.Errorf("decode problem meta invalidation failed: %v", err)
			continue
		}
		if event.ProblemID <= 0 {
			logger.Errorf("ignore problem meta invalidation with invalid problem_id=%d", event.ProblemID)
			continue
		}

		s.invalidator.InvalidateProblemMeta(event.ProblemID)
		logger.Infof("problem meta cache invalidated problem_id=%d version=%d", event.ProblemID, event.Version)
	}
}
