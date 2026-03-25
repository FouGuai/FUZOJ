package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const contestRankOutboxTable = "`contest_rank_outbox`"
const contestRankOutboxLockTable = "`contest_rank_outbox_lock`"

const (
	outboxStatusPending    = 0
	outboxStatusProcessing = 1
	outboxStatusSent       = 2
)

// RankOutboxEvent stores pending rank updates for MQ publishing.
type RankOutboxEvent struct {
	ID          int64
	ContestID   string
	EventKey    string
	Payload     string
	Status      int
	RetryCount  int
	NextRetryAt time.Time
	OwnerID     string
	LeaseUntil  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type rankOutboxEventRow struct {
	ID          int64          `db:"id"`
	ContestID   string         `db:"contest_id"`
	EventKey    string         `db:"event_key"`
	Payload     string         `db:"payload"`
	Status      int            `db:"status"`
	RetryCount  int            `db:"retry_count"`
	NextRetryAt sql.NullTime   `db:"next_retry_at"`
	OwnerID     sql.NullString `db:"owner_id"`
	LeaseUntil  sql.NullTime   `db:"lease_until"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
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
	if event.ContestID == "" {
		return errors.New("contest id is required")
	}
	now := time.Now()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = now
	}
	if event.Status == 0 {
		event.Status = outboxStatusPending
	}
	if event.NextRetryAt.IsZero() {
		event.NextRetryAt = now
	}
	query := "insert into " + contestRankOutboxTable + " (contest_id, event_key, payload, status, retry_count, next_retry_at, owner_id, lease_until, created_at, updated_at) " +
		"values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := r.conn.ExecCtx(ctx, query,
		event.ContestID,
		event.EventKey,
		event.Payload,
		event.Status,
		event.RetryCount,
		nullTime(event.NextRetryAt),
		nullString(event.OwnerID),
		nullTime(event.LeaseUntil),
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
	var rows []rankOutboxEventRow
	query := "select id, contest_id, event_key, payload, status, retry_count, next_retry_at, owner_id, lease_until, created_at, updated_at " +
		"from " + contestRankOutboxTable + " where status = ? and (next_retry_at is null or next_retry_at <= ?) " +
		"order by id asc limit ?"
	if err := r.conn.QueryRowsCtx(ctx, &rows, query, outboxStatusPending, time.Now(), limit); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return convertOutboxRows(rows), nil
}

func (r *RankOutboxRepository) MarkSent(ctx context.Context, ids []int64) error {
	return r.MarkSentByOwner(ctx, "", ids)
}

func (r *RankOutboxRepository) MarkSentByOwner(ctx context.Context, ownerID string, ids []int64) error {
	if r == nil || r.conn == nil {
		return errors.New("rank outbox repository is not configured")
	}
	if len(ids) == 0 {
		return nil
	}
	query := "update " + contestRankOutboxTable + " set status = ?, owner_id = null, lease_until = null, updated_at = ? " +
		"where status = ?"
	args := make([]any, 0, len(ids)+4)
	args = append(args, outboxStatusSent, time.Now(), outboxStatusProcessing)
	if ownerID != "" {
		query += " and owner_id = ?"
		args = append(args, ownerID)
	}
	query += " and id in (" + placeholders(len(ids)) + ")"
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := r.conn.ExecCtx(ctx, query, args...)
	return err
}

func (r *RankOutboxRepository) MarkFailed(ctx context.Context, event RankOutboxEvent, nextRetry time.Time) error {
	return r.MarkFailedWithRetry(ctx, "", event.RetryCount, []int64{event.ID}, nextRetry)
}

func (r *RankOutboxRepository) MarkFailedWithRetry(ctx context.Context, ownerID string, retryCount int, ids []int64, nextRetry time.Time) error {
	if r == nil || r.conn == nil {
		return errors.New("rank outbox repository is not configured")
	}
	if len(ids) == 0 {
		return nil
	}
	if nextRetry.IsZero() {
		nextRetry = time.Now()
	}
	query := "update " + contestRankOutboxTable + " set status = ?, retry_count = ?, next_retry_at = ?, owner_id = null, lease_until = null, updated_at = ? " +
		"where status = ? and retry_count = ?"
	args := make([]any, 0, len(ids)+7)
	args = append(args, outboxStatusPending, retryCount+1, nullTime(nextRetry), time.Now(), outboxStatusProcessing, retryCount)
	if ownerID != "" {
		query += " and owner_id = ?"
		args = append(args, ownerID)
	}
	query += " and id in (" + placeholders(len(ids)) + ")"
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := r.conn.ExecCtx(ctx, query, args...)
	return err
}

func (r *RankOutboxRepository) ListPendingContests(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if r == nil || r.conn == nil {
		return nil, errors.New("rank outbox repository is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	var contests []string
	query := "select distinct contest_id from " + contestRankOutboxTable +
		" where status = ? and (next_retry_at is null or next_retry_at <= ?) order by contest_id asc limit ?"
	if err := r.conn.QueryRowsCtx(ctx, &contests, query, outboxStatusPending, now, limit); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return contests, nil
}

func (r *RankOutboxRepository) AcquireContestLease(ctx context.Context, contestID, ownerID string, leaseDuration time.Duration) (bool, error) {
	if r == nil || r.conn == nil {
		return false, errors.New("rank outbox repository is not configured")
	}
	if contestID == "" || ownerID == "" {
		return false, errors.New("contest id and owner id are required")
	}
	if leaseDuration <= 0 {
		leaseDuration = 5 * time.Second
	}
	now := time.Now()
	leaseUntil := now.Add(leaseDuration)
	insertQuery := "insert ignore into " + contestRankOutboxLockTable + " (contest_id, owner_id, lease_until, updated_at) values (?, ?, ?, ?)"
	if _, err := r.conn.ExecCtx(ctx, insertQuery, contestID, ownerID, leaseUntil, now); err != nil {
		return false, err
	}
	updateQuery := "update " + contestRankOutboxLockTable + " set owner_id = ?, lease_until = ?, updated_at = ? " +
		"where contest_id = ? and (owner_id = ? or lease_until <= ?)"
	res, err := r.conn.ExecCtx(ctx, updateQuery, ownerID, leaseUntil, now, contestID, ownerID, now)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *RankOutboxRepository) RenewContestLease(ctx context.Context, contestID, ownerID string, leaseDuration time.Duration) error {
	if r == nil || r.conn == nil {
		return errors.New("rank outbox repository is not configured")
	}
	if contestID == "" || ownerID == "" {
		return errors.New("contest id and owner id are required")
	}
	if leaseDuration <= 0 {
		leaseDuration = 5 * time.Second
	}
	now := time.Now()
	query := "update " + contestRankOutboxLockTable + " set lease_until = ?, updated_at = ? where contest_id = ? and owner_id = ?"
	_, err := r.conn.ExecCtx(ctx, query, now.Add(leaseDuration), now, contestID, ownerID)
	return err
}

func (r *RankOutboxRepository) ReleaseContestLease(ctx context.Context, contestID, ownerID string) error {
	if r == nil || r.conn == nil {
		return errors.New("rank outbox repository is not configured")
	}
	if contestID == "" || ownerID == "" {
		return errors.New("contest id and owner id are required")
	}
	now := time.Now()
	query := "update " + contestRankOutboxLockTable + " set lease_until = ?, updated_at = ? where contest_id = ? and owner_id = ?"
	_, err := r.conn.ExecCtx(ctx, query, now, now, contestID, ownerID)
	return err
}

func (r *RankOutboxRepository) ClaimByContest(ctx context.Context, contestID, ownerID string, limit int, leaseDuration time.Duration) ([]RankOutboxEvent, error) {
	if r == nil || r.conn == nil {
		return nil, errors.New("rank outbox repository is not configured")
	}
	if contestID == "" || ownerID == "" {
		return nil, errors.New("contest id and owner id are required")
	}
	if limit <= 0 {
		limit = 100
	}
	if leaseDuration <= 0 {
		leaseDuration = 5 * time.Second
	}
	now := time.Now()
	leaseUntil := now.Add(leaseDuration)
	updateQuery := "update " + contestRankOutboxTable + " set status = ?, owner_id = ?, lease_until = ?, updated_at = ? " +
		"where contest_id = ? and status = ? and (next_retry_at is null or next_retry_at <= ?) order by id asc limit ?"
	if _, err := r.conn.ExecCtx(ctx, updateQuery, outboxStatusProcessing, ownerID, leaseUntil, now, contestID, outboxStatusPending, now, limit); err != nil {
		return nil, err
	}
	var rows []rankOutboxEventRow
	selectQuery := "select id, contest_id, event_key, payload, status, retry_count, next_retry_at, owner_id, lease_until, created_at, updated_at " +
		"from " + contestRankOutboxTable + " where contest_id = ? and status = ? and owner_id = ? order by id asc limit ?"
	if err := r.conn.QueryRowsCtx(ctx, &rows, selectQuery, contestID, outboxStatusProcessing, ownerID, limit); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return convertOutboxRows(rows), nil
}

func (r *RankOutboxRepository) RequeueExpiredProcessing(ctx context.Context, now time.Time, limit int) (int64, error) {
	if r == nil || r.conn == nil {
		return 0, errors.New("rank outbox repository is not configured")
	}
	if limit <= 0 {
		limit = 200
	}
	query := "update " + contestRankOutboxTable + " set status = ?, owner_id = null, lease_until = null, updated_at = ? " +
		"where status = ? and lease_until <= ? order by lease_until asc limit ?"
	res, err := r.conn.ExecCtx(ctx, query, outboxStatusPending, now, outboxStatusProcessing, now, limit)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (r *RankOutboxRepository) DeleteSentBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if r == nil || r.conn == nil {
		return 0, errors.New("rank outbox repository is not configured")
	}
	if limit <= 0 {
		limit = 200
	}
	query := "delete from " + contestRankOutboxTable + " where status = ? and updated_at < ? order by updated_at asc limit ?"
	res, err := r.conn.ExecCtx(ctx, query, outboxStatusSent, cutoff, limit)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
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

func (e RankOutboxEvent) ValidateForClaim(contestID, ownerID string) error {
	if e.ContestID != contestID {
		return fmt.Errorf("unexpected contest id: %s", e.ContestID)
	}
	if e.OwnerID != ownerID {
		return fmt.Errorf("unexpected owner id: %s", e.OwnerID)
	}
	return nil
}

func convertOutboxRows(rows []rankOutboxEventRow) []RankOutboxEvent {
	if len(rows) == 0 {
		return nil
	}
	events := make([]RankOutboxEvent, 0, len(rows))
	for _, row := range rows {
		event := RankOutboxEvent{
			ID:         row.ID,
			ContestID:  row.ContestID,
			EventKey:   row.EventKey,
			Payload:    row.Payload,
			Status:     row.Status,
			RetryCount: row.RetryCount,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		}
		if row.NextRetryAt.Valid {
			event.NextRetryAt = row.NextRetryAt.Time
		}
		if row.OwnerID.Valid {
			event.OwnerID = row.OwnerID.String
		}
		if row.LeaseUntil.Valid {
			event.LeaseUntil = row.LeaseUntil.Time
		}
		events = append(events, event)
	}
	return events
}
