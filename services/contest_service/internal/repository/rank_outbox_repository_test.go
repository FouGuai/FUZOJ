package repository

import (
	"database/sql"
	"testing"
	"time"
)

func TestConvertOutboxRowsWithNullFields(t *testing.T) {
	now := time.Now()
	rows := []rankOutboxEventRow{
		{
			ID:         1,
			ContestID:  "c1",
			EventKey:   "c1:u1:1",
			KafkaKey:   "c1",
			Payload:    "{}",
			Status:     "pending",
			RetryCount: 0,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}

	events := convertOutboxRows(rows)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].NextRetryAt.IsZero() {
		t.Fatalf("expected zero next retry time, got %v", events[0].NextRetryAt)
	}
	if events[0].OwnerID != "" {
		t.Fatalf("expected empty owner id, got %s", events[0].OwnerID)
	}
	if !events[0].LeaseUntil.IsZero() {
		t.Fatalf("expected zero lease time, got %v", events[0].LeaseUntil)
	}
}

func TestConvertOutboxRowsWithValidFields(t *testing.T) {
	now := time.Now()
	next := now.Add(time.Minute)
	lease := now.Add(2 * time.Minute)
	rows := []rankOutboxEventRow{
		{
			ID:          2,
			ContestID:   "c2",
			EventKey:    "c2:u1:2",
			KafkaKey:    "c2",
			Payload:     `{"ok":true}`,
			Status:      "processing",
			RetryCount:  1,
			NextRetryAt: sql.NullTime{Time: next, Valid: true},
			OwnerID:     sql.NullString{String: "owner", Valid: true},
			LeaseUntil:  sql.NullTime{Time: lease, Valid: true},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	events := convertOutboxRows(rows)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].NextRetryAt.Equal(next) {
		t.Fatalf("unexpected next retry time: %v", events[0].NextRetryAt)
	}
	if events[0].OwnerID != "owner" {
		t.Fatalf("unexpected owner id: %s", events[0].OwnerID)
	}
	if !events[0].LeaseUntil.Equal(lease) {
		t.Fatalf("unexpected lease time: %v", events[0].LeaseUntil)
	}
}
