package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProblemModel = (*customProblemModel)(nil)

type (
	// ProblemModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProblemModel.
	ProblemModel interface {
		problemModel
		WithSession(session sqlx.Session) ProblemModel
		Exists(ctx context.Context, problemID int64) (bool, error)
		DeleteByID(ctx context.Context, problemID int64) (bool, error)
	}

	customProblemModel struct {
		*defaultProblemModel
	}
)

// NewProblemModel returns a model for the database table.
func NewProblemModel(conn sqlx.SqlConn) ProblemModel {
	return &customProblemModel{
		defaultProblemModel: newProblemModel(conn),
	}
}

func (m *customProblemModel) WithSession(session sqlx.Session) ProblemModel {
	if session == nil {
		return m
	}
	return NewProblemModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customProblemModel) Exists(ctx context.Context, problemID int64) (bool, error) {
	if problemID <= 0 {
		return false, fmt.Errorf("problemID is required")
	}
	var exists int
	if err := m.conn.QueryRowCtx(ctx, &exists, "SELECT 1 FROM problem WHERE id = ? LIMIT 1", problemID); err != nil {
		if err == ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (m *customProblemModel) DeleteByID(ctx context.Context, problemID int64) (bool, error) {
	if problemID <= 0 {
		return false, fmt.Errorf("problemID is required")
	}
	res, err := m.conn.ExecCtx(ctx, "DELETE FROM problem WHERE id = ?", problemID)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
