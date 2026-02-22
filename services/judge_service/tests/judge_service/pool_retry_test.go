package judge_service_test

import (
	"context"
	"testing"
	"time"

	"fuzoj/internal/common/mq"
	"fuzoj/services/judge_service/internal/logic"
)

type publishedMessage struct {
	topic string
	msg   *mq.Message
}

type fakeQueue struct {
	published []publishedMessage
}

func (f *fakeQueue) Publish(ctx context.Context, topic string, message *mq.Message) error {
	f.published = append(f.published, publishedMessage{topic: topic, msg: message})
	return nil
}

func (f *fakeQueue) PublishBatch(ctx context.Context, topic string, messages []*mq.Message) error {
	for _, msg := range messages {
		f.published = append(f.published, publishedMessage{topic: topic, msg: msg})
	}
	return nil
}

func (f *fakeQueue) Subscribe(ctx context.Context, topic string, handler mq.HandlerFunc) error {
	return nil
}

func (f *fakeQueue) SubscribeWithOptions(ctx context.Context, topic string, handler mq.HandlerFunc, opts *mq.SubscribeOptions) error {
	return nil
}

func (f *fakeQueue) Start() error { return nil }

func (f *fakeQueue) Stop() error { return nil }

func (f *fakeQueue) Pause() error { return nil }

func (f *fakeQueue) Resume() error { return nil }

func (f *fakeQueue) Ping(ctx context.Context) error { return nil }

func (f *fakeQueue) Close() error { return nil }

func TestParsePoolRetryCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{name: "empty", headers: nil, want: 0},
		{name: "missing", headers: map[string]string{}, want: 0},
		{name: "invalid", headers: map[string]string{"x-pool-retry": "bad"}, want: 0},
		{name: "negative", headers: map[string]string{"x-pool-retry": "-1"}, want: 0},
		{name: "ok", headers: map[string]string{"x-pool-retry": "3"}, want: 3},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := logic.ParsePoolRetryCount(tt.headers); got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestComputePoolBackoff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		retryCount int
		base       time.Duration
		max        time.Duration
		want       time.Duration
	}{
		{name: "base", retryCount: 0, base: time.Second, max: 30 * time.Second, want: time.Second},
		{name: "double", retryCount: 1, base: time.Second, max: 30 * time.Second, want: 2 * time.Second},
		{name: "quad", retryCount: 2, base: time.Second, max: 30 * time.Second, want: 4 * time.Second},
		{name: "capped", retryCount: 10, base: time.Second, max: 30 * time.Second, want: 30 * time.Second},
		{name: "no-base", retryCount: 3, base: 0, max: 30 * time.Second, want: 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := logic.ComputePoolBackoff(tt.retryCount, tt.base, tt.max); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestRequeueForPoolFull(t *testing.T) {
	t.Parallel()
	t.Run("publish-retry", func(t *testing.T) {
		t.Parallel()
		queue := &fakeQueue{}
		msg := mq.NewMessage([]byte("payload"))
		msg.Headers["x-pool-retry"] = "1"
		if err := logic.RequeueForPoolFull(context.Background(), queue, "judge.retry", "judge.dead", 5, 0, 0, msg); err != nil {
			t.Fatalf("requeue failed: %v", err)
		}
		if len(queue.published) != 1 {
			t.Fatalf("expected 1 published message, got %d", len(queue.published))
		}
		got := queue.published[0]
		if got.topic != "judge.retry" {
			t.Fatalf("expected retry topic, got %s", got.topic)
		}
		if got.msg.Headers["x-pool-retry"] != "2" {
			t.Fatalf("expected retry count 2, got %s", got.msg.Headers["x-pool-retry"])
		}
		if got.msg.ID != "" {
			t.Fatalf("expected empty message ID")
		}
	})

	t.Run("publish-deadletter", func(t *testing.T) {
		t.Parallel()
		queue := &fakeQueue{}
		msg := mq.NewMessage([]byte("payload"))
		msg.Headers["x-pool-retry"] = "5"
		if err := logic.RequeueForPoolFull(context.Background(), queue, "judge.retry", "judge.dead", 5, 0, 0, msg); err != nil {
			t.Fatalf("deadletter failed: %v", err)
		}
		if len(queue.published) != 1 {
			t.Fatalf("expected 1 published message, got %d", len(queue.published))
		}
		got := queue.published[0]
		if got.topic != "judge.dead" {
			t.Fatalf("expected deadletter topic, got %s", got.topic)
		}
		if got.msg.Headers["x-pool-retry"] != "5" {
			t.Fatalf("expected retry count 5, got %s", got.msg.Headers["x-pool-retry"])
		}
	})
}
