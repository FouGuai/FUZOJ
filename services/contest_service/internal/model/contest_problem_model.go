package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ContestProblem struct {
	ContestId string `db:"contest_id"`
	ProblemId int64  `db:"problem_id"`
	Order     int    `db:"order"`
	Score     int    `db:"score"`
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

func (m *ContestProblemModel) Upsert(ctx context.Context, row ContestProblem) error {
	query := "insert into " + m.table + " (`contest_id`, `problem_id`, `order`, `score`, `visible`, `version`) values (?, ?, ?, ?, ?, ?) " +
		"on duplicate key update `order`=values(`order`), `score`=values(`score`), `visible`=values(`visible`), `version`=values(`version`)"
	_, err := m.conn.ExecCtx(ctx, query, row.ContestId, row.ProblemId, row.Order, row.Score, row.Visible, row.Version)
	return err
}

func (m *ContestProblemModel) Update(ctx context.Context, row ContestProblem) (int64, error) {
	query := "update " + m.table + " set `order`=?, `score`=?, `visible`=?, `version`=? where `contest_id`=? and `problem_id`=?"
	res, err := m.conn.ExecCtx(ctx, query, row.Order, row.Score, row.Visible, row.Version, row.ContestId, row.ProblemId)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *ContestProblemModel) Delete(ctx context.Context, contestID string, problemID int64) (int64, error) {
	query := "delete from " + m.table + " where `contest_id`=? and `problem_id`=?"
	res, err := m.conn.ExecCtx(ctx, query, contestID, problemID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *ContestProblemModel) ListByContest(ctx context.Context, contestID string) ([]ContestProblem, error) {
	query := "select `contest_id`, `problem_id`, `order`, `score`, `visible`, `version` from " + m.table + " where `contest_id`=? order by `order` asc, `problem_id` asc"
	var rows []ContestProblem
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, contestID); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return rows, nil
}
