package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Contest struct {
	ContestId  string    `db:"contest_id"`
	Status     string    `db:"status"`
	Visibility string    `db:"visibility"`
	StartAt    time.Time `db:"start_at"`
	EndAt      time.Time `db:"end_at"`
	RuleJSON   string    `db:"rule_json"`
}

type ContestModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewContestModel(conn sqlx.SqlConn) *ContestModel {
	return &ContestModel{
		conn:  conn,
		table: "`contests`",
	}
}

func (m *ContestModel) FindMeta(ctx context.Context, contestID string) (Contest, error) {
	var resp Contest
	query := "select contest_id, status, visibility, start_at, end_at, rule_json from " + m.table + " where `contest_id` = ? limit 1"
	err := m.conn.QueryRowCtx(ctx, &resp, query, contestID)
	return resp, err
}
