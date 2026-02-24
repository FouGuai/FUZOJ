package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProblemStatementModel = (*customProblemStatementModel)(nil)

type (
	// ProblemStatementModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProblemStatementModel.
	ProblemStatementModel interface {
		WithSession(session sqlx.Session) ProblemStatementModel
		FindLatestPublishedByProblemID(ctx context.Context, problemID int64) (*ProblemStatement, error)
		FindByProblemIDVersion(ctx context.Context, problemID int64, version int64) (*ProblemStatement, error)
		Upsert(ctx context.Context, statement *ProblemStatement) error
	}

	customProblemStatementModel struct {
		conn sqlx.SqlConn
	}
)

// ProblemStatement represents a problem statement row.
type ProblemStatement struct {
	Id               int64     `db:"id"`
	ProblemVersionId int64     `db:"problem_version_id"`
	ProblemId        int64     `db:"problem_id"`
	Version          int64     `db:"version"`
	StatementMd      string    `db:"statement_md"`
	StatementHash    string    `db:"statement_hash"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// NewProblemStatementModel returns a model for the database table.
func NewProblemStatementModel(conn sqlx.SqlConn) ProblemStatementModel {
	return &customProblemStatementModel{conn: conn}
}

func (m *customProblemStatementModel) WithSession(session sqlx.Session) ProblemStatementModel {
	if session == nil {
		return m
	}
	return NewProblemStatementModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customProblemStatementModel) FindLatestPublishedByProblemID(ctx context.Context, problemID int64) (*ProblemStatement, error) {
	if problemID <= 0 {
		return nil, fmt.Errorf("problemID is required")
	}
	query := `
		SELECT ps.id, ps.problem_version_id, ps.problem_id, ps.version, ps.statement_md, ps.statement_hash, ps.created_at, ps.updated_at
		FROM problem_statement ps
		JOIN problem_version pv ON pv.id = ps.problem_version_id
		WHERE pv.problem_id = ? AND pv.state = 1
		ORDER BY pv.version DESC
		LIMIT 1`
	var resp ProblemStatement
	err := m.conn.QueryRowCtx(ctx, &resp, query, problemID)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customProblemStatementModel) FindByProblemIDVersion(ctx context.Context, problemID int64, version int64) (*ProblemStatement, error) {
	if problemID <= 0 || version <= 0 {
		return nil, fmt.Errorf("problemID and version are required")
	}
	query := `
		SELECT id, problem_version_id, problem_id, version, statement_md, statement_hash, created_at, updated_at
		FROM problem_statement
		WHERE problem_id = ? AND version = ?`
	var resp ProblemStatement
	err := m.conn.QueryRowCtx(ctx, &resp, query, problemID, version)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customProblemStatementModel) Upsert(ctx context.Context, statement *ProblemStatement) error {
	if statement == nil {
		return fmt.Errorf("statement is nil")
	}
	if statement.ProblemId <= 0 || statement.Version <= 0 || statement.ProblemVersionId <= 0 {
		return fmt.Errorf("problemID, version, and problemVersionID are required")
	}
	_, err := m.conn.ExecCtx(ctx, `
		INSERT INTO problem_statement (problem_version_id, problem_id, version, statement_md, statement_hash)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE statement_md = VALUES(statement_md), statement_hash = VALUES(statement_hash), updated_at = CURRENT_TIMESTAMP(3)`,
		statement.ProblemVersionId, statement.ProblemId, statement.Version, statement.StatementMd, statement.StatementHash,
	)
	return err
}
