package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_service/internal/consumer"
	"fuzoj/services/rank_service/internal/pmodel"
)

func TestRankUpdateConsumer_ReturnsErrorOnAddFailure(t *testing.T) {
	repo := newFakeUpdateRepo(0)
	batcher := consumer.NewUpdateBatcher(repo, nil, 1, time.Hour, time.Second)
	for i := 0; i < 4; i++ {
		if err := batcher.Add(context.Background(), pmodel.RankUpdateEvent{
			ContestID: "c1",
			MemberID:  "m1",
			Version:   "1",
		}); err != nil {
			t.Fatalf("unexpected add error: %v", err)
		}
	}
	consumer := consumer.NewRankUpdateConsumer(batcher, 10*time.Millisecond)
	event := pmodel.RankUpdateEvent{
		ContestID: "c1",
		MemberID:  "m1",
		Version:   "2",
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event failed: %v", err)
	}
	if err := consumer.Consume(context.Background(), "", string(payload)); err == nil {
		t.Fatalf("expected enqueue timeout error")
	} else if appErr.GetCode(err) != appErr.Timeout {
		t.Fatalf("unexpected error code: %v", err)
	}
}
