package repository

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const contestRankOutboxTable = "`contest_rank_outbox`"

// RankOutboxEvent stores pending rank updates for MQ publishing.
type RankOutboxEvent struct {
	ID          int64
	EventKey    string
	KafkaKey    string
	Payload     string
	Status      string
	RetryCount  int
	NextRetryAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RankOutboxRepository handles outbox persistence.
type RankOutboxRepository struct {
	conn sqlRunner
}

func NewRankOutboxRepository(conn sqlRunner) *RankOutboxRepository {
	return &RankOutboxRepository{conn: conn}
}

func (r *RankOutboxRepository) Enqueue(ctx context.Context, event RankOutboxEvent) error {
	if r == nil || r.conn == nil {
		return errors.New("rank outbox repository is not configured")
	}
	now := time.Now()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = now
	}
	if event.Status == "" {
		event.Status = "pending"
	}
	query := "insert into " + contestRankOutboxTable + " (event_key, kafka_key, payload, status, retry_count, next_retry_at, created_at, updated_at) " +
		"values (?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := r.conn.ExecCtx(ctx, query,
		event.EventKey,
		event.KafkaKey,
		event.Payload,
		event.Status,
		event.RetryCount,
		nullTime(event.NextRetryAt),
		event.CreatedAt,
		event.UpdatedAt,
	)
	return err
}

func (r *RankOutboxRepository) ListPending(ctx context.Context, limit int) ([]RankOutboxEvent, error) {
	if r == nil || r.conn == nil {
		return nil, errors.New("rank outbox repository is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	var resp []RankOutboxEvent
	query := "select id, event_key, kafka_key, payload, status, retry_count, next_retry_at, created_at, updated_at " +
		"from " + contestRankOutboxTable + " where status = 'pending' and (next_retry_at is null or next_retry_at <= ?) " +
		"order by id asc limit ?"
	if err := r.conn.QueryRowsCtx(ctx, &resp, query, time.Now(), limit); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return resp, nil
}

func (r *RankOutboxRepository) MarkSent(ctx context.Context, ids []int64) error {
	if r == nil || r.conn == nil {
		return errors.New("rank outbox repository is not configured")
	}
	if len(ids) == 0 {
		return nil
	}
	query := "update " + contestRankOutboxTable + " set status = 'sent', updated_at = ? where id in (" + placeholders(len(ids)) + ")"
	args := make([]any, 0, len(ids)+1)
	args = append(args, time.Now())
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := r.conn.ExecCtx(ctx, query, args...)
	return err
}

func (r *RankOutboxRepository) MarkFailed(ctx context.Context, event RankOutboxEvent, nextRetry time.Time) error {
	if r == nil || r.conn == nil {
		return errors.New("rank outbox repository is not configured")
	}
	query := "update " + contestRankOutboxTable + " set retry_count = ?, next_retry_at = ?, updated_at = ? where id = ?"
	_, err := r.conn.ExecCtx(ctx, query, event.RetryCount+1, nullTime(nextRetry), time.Now(), event.ID)
	return err
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, 0, n*2)
	for i := 0; i < n; i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '?')
	}
	return string(buf)
}
