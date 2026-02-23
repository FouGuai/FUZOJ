package judge_service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"fuzoj/services/judge_service/internal/logic/judge_app"
	"fuzoj/services/judge_service/internal/pmodel"
)

type publishedMessage struct {
	key   string
	value string
}

type fakePusher struct {
	published []publishedMessage
}

func (f *fakePusher) PushWithKey(ctx context.Context, key, value string) error {
	f.published = append(f.published, publishedMessage{key: key, value: value})
	return nil
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
			if got := judge_app.ComputePoolBackoff(tt.retryCount, tt.base, tt.max); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestRequeueForPoolFull(t *testing.T) {
	t.Parallel()
	t.Run("publish-retry", func(t *testing.T) {
		t.Parallel()
		retryPusher := &fakePusher{}
		payload := pmodel.JudgeMessage{
			SubmissionID: "sub-1",
			PoolRetry:    1,
		}
		if err := judge_app.RequeueForPoolFull(context.Background(), retryPusher, nil, "judge.retry", "judge.dead", 5, 0, 0, payload); err != nil {
			t.Fatalf("requeue failed: %v", err)
		}
		if len(retryPusher.published) != 1 {
			t.Fatalf("expected 1 published message, got %d", len(retryPusher.published))
		}
		got := retryPusher.published[0]
		if got.key != "sub-1" {
			t.Fatalf("expected key sub-1, got %s", got.key)
		}
		var decoded pmodel.JudgeMessage
		if err := json.Unmarshal([]byte(got.value), &decoded); err != nil {
			t.Fatalf("decode requeue payload failed: %v", err)
		}
		if decoded.PoolRetry != 2 {
			t.Fatalf("expected retry count 2, got %d", decoded.PoolRetry)
		}
		if decoded.CreatedAt == 0 {
			t.Fatalf("expected created_at to be set")
		}
	})

	t.Run("publish-deadletter", func(t *testing.T) {
		t.Parallel()
		deadPusher := &fakePusher{}
		payload := pmodel.JudgeMessage{
			SubmissionID: "sub-2",
			PoolRetry:    5,
		}
		if err := judge_app.RequeueForPoolFull(context.Background(), &fakePusher{}, deadPusher, "judge.retry", "judge.dead", 5, 0, 0, payload); err != nil {
			t.Fatalf("deadletter failed: %v", err)
		}
		if len(deadPusher.published) != 1 {
			t.Fatalf("expected 1 published message, got %d", len(deadPusher.published))
		}
		got := deadPusher.published[0]
		if got.key != "sub-2" {
			t.Fatalf("expected key sub-2, got %s", got.key)
		}
		var decoded pmodel.JudgeMessage
		if err := json.Unmarshal([]byte(got.value), &decoded); err != nil {
			t.Fatalf("decode dead letter payload failed: %v", err)
		}
		if decoded.PoolRetry != 5 {
			t.Fatalf("expected retry count 5, got %d", decoded.PoolRetry)
		}
	})
}
