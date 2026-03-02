package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ContestProblem struct {
	ContestId string `db:"contest_id"`
	ProblemId int64  `db:"problem_id"`
	Visible   bool   `db:"visible"`
	Version   int32  `db:"version"`
}

type ContestProblemModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewContestProblemModel(conn sqlx.SqlConn) *ContestProblemModel {
	return &ContestProblemModel{
		conn:  conn,
		table: "`contest_problems`",
	}
}

func (m *ContestProblemModel) Exists(ctx context.Context, contestID string, problemID int64) (bool, error) {
	query := "select 1 from " + m.table + " where `contest_id` = ? and `problem_id` = ? limit 1"
	var exists int64
	err := m.conn.QueryRowCtx(ctx, &exists, query, contestID, problemID)
	if err == sqlx.ErrNotFound {
		return false, nil
	}
	return err == nil, err
}
