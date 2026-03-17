package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SubmissionsModel = (*customSubmissionsModel)(nil)

type (
	// SubmissionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSubmissionsModel.
	SubmissionsModel interface {
		submissionsModel
		WithSession(session sqlx.Session) SubmissionsModel
		FindFinalStatus(ctx context.Context, submissionID string) (string, error)
		HasFinalStatus(ctx context.Context, submissionID string) (bool, error)
		FindFinalStatusBatch(ctx context.Context, submissionIDs []string) ([]SubmissionFinalStatus, error)
		UpdateFinalStatus(ctx context.Context, submissionID string, payload string, finishedAt time.Time) (sql.Result, error)
	}

	customSubmissionsModel struct {
		*defaultSubmissionsModel
	}
)

// SubmissionFinalStatus represents a submission with final status payload.
type SubmissionFinalStatus struct {
	SubmissionID string `db:"submission_id"`
	FinalStatus  string `db:"final_status"`
}

// NewSubmissionsModel returns a model for the database table.
func NewSubmissionsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SubmissionsModel {
	return &customSubmissionsModel{
		defaultSubmissionsModel: newSubmissionsModel(conn, c, opts...),
	}
}

func (m *customSubmissionsModel) WithSession(session sqlx.Session) SubmissionsModel {
	if session == nil {
		return m
	}
	return &customSubmissionsModel{
		defaultSubmissionsModel: &defaultSubmissionsModel{
			CachedConn: m.CachedConn.WithSession(session),
			table:      m.table,
		},
	}
}

func (m *customSubmissionsModel) FindFinalStatus(ctx context.Context, submissionID string) (string, error) {
	query := fmt.Sprintf("select `final_status` from %s where `submission_id` = ? and `final_status` is not null limit 1", m.table)
	var resp struct {
		FinalStatus string `db:"final_status"`
	}
	if err := m.QueryRowNoCacheCtx(ctx, &resp, query, submissionID); err != nil {
		if err == sqlx.ErrNotFound {
			return "", ErrNotFound
		}
		return "", err
	}
	return resp.FinalStatus, nil
}

func (m *customSubmissionsModel) HasFinalStatus(ctx context.Context, submissionID string) (bool, error) {
	query := fmt.Sprintf("select `final_status_at` from %s where `submission_id` = ? and `final_status_at` is not null limit 1", m.table)
	var resp struct {
		FinalStatusAt time.Time `db:"final_status_at"`
	}
	if err := m.QueryRowNoCacheCtx(ctx, &resp, query, submissionID); err != nil {
		if err == sqlx.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (m *customSubmissionsModel) FindFinalStatusBatch(ctx context.Context, submissionIDs []string) ([]SubmissionFinalStatus, error) {
	if len(submissionIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, 0, len(submissionIDs))
	args := make([]any, 0, len(submissionIDs))
	for _, id := range submissionIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf(
		"select `submission_id`, `final_status` from %s where `submission_id` in (%s) and `final_status` is not null",
		m.table,
		strings.Join(placeholders, ","),
	)
	var resp []SubmissionFinalStatus
	if err := m.QueryRowsNoCacheCtx(ctx, &resp, query, args...); err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *customSubmissionsModel) UpdateFinalStatus(ctx context.Context, submissionID string, payload string, finishedAt time.Time) (sql.Result, error) {
	query := fmt.Sprintf("update %s set `final_status` = ?, `final_status_at` = ? where `submission_id` = ?", m.table)
	return m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, payload, finishedAt, submissionID)
	})
}
