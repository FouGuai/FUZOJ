package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProblemVersionSeqModel = (*customProblemVersionSeqModel)(nil)

type (
	// ProblemVersionSeqModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProblemVersionSeqModel.
	ProblemVersionSeqModel interface {
		problemVersionSeqModel
		WithSession(session sqlx.Session) ProblemVersionSeqModel
		AllocateNextVersion(ctx context.Context, problemID int64) (int64, error)
	}

	customProblemVersionSeqModel struct {
		*defaultProblemVersionSeqModel
	}
)

// NewProblemVersionSeqModel returns a model for the database table.
func NewProblemVersionSeqModel(conn sqlx.SqlConn) ProblemVersionSeqModel {
	return &customProblemVersionSeqModel{
		defaultProblemVersionSeqModel: newProblemVersionSeqModel(conn),
	}
}

func (m *customProblemVersionSeqModel) WithSession(session sqlx.Session) ProblemVersionSeqModel {
	if session == nil {
		return m
	}
	return NewProblemVersionSeqModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customProblemVersionSeqModel) AllocateNextVersion(ctx context.Context, problemID int64) (int64, error) {
	if problemID <= 0 {
		return 0, fmt.Errorf("problemID is required")
	}
	_, err := m.conn.ExecCtx(ctx, "INSERT IGNORE INTO problem_version_seq (problem_id, next_version) VALUES (?, 1)", problemID)
	if err != nil {
		return 0, err
	}
	_, err = m.conn.ExecCtx(ctx, "UPDATE problem_version_seq SET next_version = LAST_INSERT_ID(next_version + 1) WHERE problem_id = ?", problemID)
	if err != nil {
		return 0, err
	}
	var next int64
	if err := m.conn.QueryRowCtx(ctx, &next, "SELECT LAST_INSERT_ID()"); err != nil {
		return 0, err
	}
	version := next - 1
	if version <= 0 {
		return 0, fmt.Errorf("invalid allocated version: %d", version)
	}
	return version, nil
}
