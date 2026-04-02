package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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
			Payload:    "{}",
			Status:     outboxStatusPending,
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
			Payload:     `{"ok":true}`,
			Status:      outboxStatusProcessing,
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

func TestNextResultIDTx(t *testing.T) {
	runner := &stubSQLRunner{
		queryRowFunc: func(v any, query string, args ...any) error {
			if !strings.Contains(strings.ToLower(query), "for update") {
				t.Fatalf("expected lock query, got: %s", query)
			}
			ptr, ok := v.(*int64)
			if !ok {
				t.Fatalf("expected *int64 scan target")
			}
			*ptr = 7
			return nil
		},
		execFunc: func(query string, args ...any) (stubResult, error) {
			if strings.Contains(query, "contest_rank_result_seq") && strings.Contains(strings.ToLower(query), "update") {
				return stubResult(1), nil
			}
			return stubResult(1), nil
		},
	}
	repo := NewRankOutboxRepository(runner)
	got, err := repo.NextResultIDTx(context.Background(), "c1")
	if err != nil {
		t.Fatalf("next result id failed: %v", err)
	}
	if got != 7 {
		t.Fatalf("expected result id 7, got %d", got)
	}
	if runner.execCalls < 2 {
		t.Fatalf("expected at least 2 exec calls, got %d", runner.execCalls)
	}
	if runner.queryRowCalls != 1 {
		t.Fatalf("expected 1 query row call, got %d", runner.queryRowCalls)
	}
}

func TestNextResultIDTxErrors(t *testing.T) {
	repo := NewRankOutboxRepository(nil)
	if _, err := repo.NextResultIDTx(context.Background(), "c1"); err == nil {
		t.Fatalf("expected nil repository error")
	}

	runner := &stubSQLRunner{
		execFunc: func(query string, args ...any) (stubResult, error) {
			if strings.Contains(strings.ToLower(query), "insert ignore") {
				return 0, errors.New("insert failed")
			}
			return stubResult(1), nil
		},
	}
	repo = NewRankOutboxRepository(runner)
	if _, err := repo.NextResultIDTx(context.Background(), "c1"); err == nil {
		t.Fatalf("expected insert error")
	}
	if _, err := repo.NextResultIDTx(context.Background(), ""); err == nil {
		t.Fatalf("expected empty contest id error")
	}
}

type stubSQLRunner struct {
	execCalls     int
	queryRowCalls int
	execFunc      func(query string, args ...any) (stubResult, error)
	queryRowFunc  func(v any, query string, args ...any) error
}

func (s *stubSQLRunner) ExecCtx(_ context.Context, query string, args ...any) (sql.Result, error) {
	s.execCalls++
	if s.execFunc != nil {
		res, err := s.execFunc(query, args...)
		return res, err
	}
	return stubResult(1), nil
}

func (s *stubSQLRunner) QueryRowCtx(_ context.Context, v any, query string, args ...any) error {
	s.queryRowCalls++
	if s.queryRowFunc != nil {
		return s.queryRowFunc(v, query, args...)
	}
	return nil
}

func (s *stubSQLRunner) QueryRowsCtx(_ context.Context, _ any, _ string, _ ...any) error {
	return nil
}

type stubResult int64

func (r stubResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r stubResult) RowsAffected() (int64, error) {
	return int64(r), nil
}
