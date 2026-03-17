package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const submissionDispatchOutboxTable = "`submission_dispatch_outbox`"

const (
	DispatchStatusPending    = "pending"
	DispatchStatusProcessing = "processing"
	DispatchStatusDone       = "done"
)

// SubmissionDispatchRecord stores dispatch recovery state.
type SubmissionDispatchRecord struct {
	ID           int64
	SubmissionID string
	Scene        string
	ContestID    string
	Payload      string
	Status       string
	RetryCount   int
	NextRetryAt  time.Time
	OwnerID      string
	LeaseUntil   time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type submissionDispatchRecordRow struct {
	ID           int64          `db:"id"`
	SubmissionID string         `db:"submission_id"`
	Scene        string         `db:"scene"`
	ContestID    sql.NullString `db:"contest_id"`
	Payload      string         `db:"payload"`
	Status       string         `db:"status"`
	RetryCount   int            `db:"retry_count"`
	NextRetryAt  time.Time      `db:"next_retry_at"`
	OwnerID      sql.NullString `db:"owner_id"`
	LeaseUntil   sql.NullTime   `db:"lease_until"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

// SubmissionDispatchRepository defines dispatch outbox operations.
type SubmissionDispatchRepository interface {
	Create(ctx context.Context, session sqlx.Session, record SubmissionDispatchRecord) error
	MarkDone(ctx context.Context, session sqlx.Session, submissionID string) error
	RequeueExpiredProcessing(ctx context.Context, now time.Time, limit int) (int64, error)
	ClaimDue(ctx context.Context, now time.Time, ownerID string, leaseDuration time.Duration, limit int) ([]SubmissionDispatchRecord, error)
	MarkPublished(ctx context.Context, id int64, ownerID string, nextRetryAt time.Time) error
	MarkRetry(ctx context.Context, id int64, ownerID string, retryCount int, nextRetryAt time.Time) error
}

// MySQLSubmissionDispatchRepository persists dispatch outbox records.
type MySQLSubmissionDispatchRepository struct {
	conn sqlx.SqlConn
}

// NewSubmissionDispatchRepository creates a dispatch outbox repository.
func NewSubmissionDispatchRepository(conn sqlx.SqlConn) SubmissionDispatchRepository {
	if conn == nil {
		return nil
	}
	return &MySQLSubmissionDispatchRepository{conn: conn}
}

// Create inserts a dispatch record.
func (r *MySQLSubmissionDispatchRepository) Create(ctx context.Context, session sqlx.Session, record SubmissionDispatchRecord) error {
	if r == nil || r.conn == nil {
		return errors.New("dispatch repository is not configured")
	}
	if record.SubmissionID == "" {
		return errors.New("submissionID is required")
	}
	if record.Payload == "" {
		return errors.New("payload is required")
	}
	if record.Status == "" {
		record.Status = DispatchStatusPending
	}
	now := time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	if record.NextRetryAt.IsZero() {
		record.NextRetryAt = now
	}
	query := "insert into " + submissionDispatchOutboxTable + " (submission_id, scene, contest_id, payload, status, retry_count, next_retry_at, owner_id, lease_until, created_at, updated_at) " +
		"values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	conn := sessionOrConn(r.conn, session)
	_, err := conn.ExecCtx(ctx, query,
		record.SubmissionID,
		record.Scene,
		nullString(record.ContestID),
		record.Payload,
		record.Status,
		record.RetryCount,
		record.NextRetryAt,
		nullString(record.OwnerID),
		nullTime(record.LeaseUntil),
		record.CreatedAt,
		record.UpdatedAt,
	)
	return err
}

// MarkDone marks a submission as completed for recovery.
func (r *MySQLSubmissionDispatchRepository) MarkDone(ctx context.Context, session sqlx.Session, submissionID string) error {
	if r == nil || r.conn == nil {
		return errors.New("dispatch repository is not configured")
	}
	if submissionID == "" {
		return errors.New("submissionID is required")
	}
	query := "update " + submissionDispatchOutboxTable + " set status = ?, owner_id = null, lease_until = null, updated_at = ? where submission_id = ? and status != ?"
	conn := sessionOrConn(r.conn, session)
	_, err := conn.ExecCtx(ctx, query, DispatchStatusDone, time.Now(), submissionID, DispatchStatusDone)
	return err
}

// RequeueExpiredProcessing requeues records that exceeded lease time.
func (r *MySQLSubmissionDispatchRepository) RequeueExpiredProcessing(ctx context.Context, now time.Time, limit int) (int64, error) {
	if r == nil || r.conn == nil {
		return 0, errors.New("dispatch repository is not configured")
	}
	if limit <= 0 {
		limit = 200
	}
	query := "update " + submissionDispatchOutboxTable + " set status = ?, owner_id = null, lease_until = null, updated_at = ? " +
		"where status = ? and lease_until is not null and lease_until <= ? order by lease_until asc limit ?"
	res, err := r.conn.ExecCtx(ctx, query, DispatchStatusPending, now, DispatchStatusProcessing, now, limit)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// ClaimDue marks due pending rows as processing and returns claimed records.
func (r *MySQLSubmissionDispatchRepository) ClaimDue(ctx context.Context, now time.Time, ownerID string, leaseDuration time.Duration, limit int) ([]SubmissionDispatchRecord, error) {
	if r == nil || r.conn == nil {
		return nil, errors.New("dispatch repository is not configured")
	}
	if ownerID == "" {
		return nil, errors.New("ownerID is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if leaseDuration <= 0 {
		leaseDuration = 5 * time.Second
	}
	leaseUntil := now.Add(leaseDuration)
	updateQuery := "update " + submissionDispatchOutboxTable + " set status = ?, owner_id = ?, lease_until = ?, updated_at = ? " +
		"where status = ? and next_retry_at <= ? order by next_retry_at asc, id asc limit ?"
	if _, err := r.conn.ExecCtx(ctx, updateQuery, DispatchStatusProcessing, ownerID, leaseUntil, now, DispatchStatusPending, now, limit); err != nil {
		return nil, err
	}
	selectQuery := "select id, submission_id, scene, contest_id, payload, status, retry_count, next_retry_at, owner_id, lease_until, created_at, updated_at " +
		"from " + submissionDispatchOutboxTable + " where status = ? and owner_id = ? order by next_retry_at asc, id asc limit ?"
	var rows []submissionDispatchRecordRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, selectQuery, DispatchStatusProcessing, ownerID, limit); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return convertDispatchRows(rows), nil
}

// MarkPublished schedules the next timeout scan after successful dispatch.
func (r *MySQLSubmissionDispatchRepository) MarkPublished(ctx context.Context, id int64, ownerID string, nextRetryAt time.Time) error {
	if r == nil || r.conn == nil {
		return errors.New("dispatch repository is not configured")
	}
	if id <= 0 {
		return errors.New("id is required")
	}
	if ownerID == "" {
		return errors.New("ownerID is required")
	}
	query := "update " + submissionDispatchOutboxTable + " set status = ?, next_retry_at = ?, owner_id = null, lease_until = null, updated_at = ? " +
		"where id = ? and status = ? and owner_id = ?"
	_, err := r.conn.ExecCtx(ctx, query, DispatchStatusPending, nextRetryAt, time.Now(), id, DispatchStatusProcessing, ownerID)
	return err
}

// MarkRetry updates retry state after dispatch failure.
func (r *MySQLSubmissionDispatchRepository) MarkRetry(ctx context.Context, id int64, ownerID string, retryCount int, nextRetryAt time.Time) error {
	if r == nil || r.conn == nil {
		return errors.New("dispatch repository is not configured")
	}
	if id <= 0 {
		return errors.New("id is required")
	}
	if ownerID == "" {
		return errors.New("ownerID is required")
	}
	query := "update " + submissionDispatchOutboxTable + " set status = ?, retry_count = ?, next_retry_at = ?, owner_id = null, lease_until = null, updated_at = ? " +
		"where id = ? and status = ? and owner_id = ?"
	_, err := r.conn.ExecCtx(ctx, query, DispatchStatusPending, retryCount, nextRetryAt, time.Now(), id, DispatchStatusProcessing, ownerID)
	return err
}

func convertDispatchRows(rows []submissionDispatchRecordRow) []SubmissionDispatchRecord {
	if len(rows) == 0 {
		return nil
	}
	out := make([]SubmissionDispatchRecord, 0, len(rows))
	for _, row := range rows {
		item := SubmissionDispatchRecord{
			ID:           row.ID,
			SubmissionID: row.SubmissionID,
			Scene:        row.Scene,
			Payload:      row.Payload,
			Status:       row.Status,
			RetryCount:   row.RetryCount,
			NextRetryAt:  row.NextRetryAt,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		}
		if row.ContestID.Valid {
			item.ContestID = row.ContestID.String
		}
		if row.OwnerID.Valid {
			item.OwnerID = row.OwnerID.String
		}
		if row.LeaseUntil.Valid {
			item.LeaseUntil = row.LeaseUntil.Time
		}
		out = append(out, item)
	}
	return out
}

func sessionOrConn(conn sqlx.SqlConn, session sqlx.Session) sqlx.Session {
	if session != nil {
		return session
	}
	return conn
}

func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func nullString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}
