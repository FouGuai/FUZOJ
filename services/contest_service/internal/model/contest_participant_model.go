package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ContestParticipant struct {
	ContestId    string         `db:"contest_id"`
	UserId       int64          `db:"user_id"`
	TeamId       sql.NullString `db:"team_id"`
	Status       string         `db:"status"`
	RegisteredAt time.Time      `db:"registered_at"`
}

type ContestParticipantModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewContestParticipantModel(conn sqlx.SqlConn) *ContestParticipantModel {
	return &ContestParticipantModel{
		conn:  conn,
		table: "`contest_participants`",
	}
}

func (m *ContestParticipantModel) Upsert(ctx context.Context, row ContestParticipant) error {
	query := "insert into " + m.table + " (`contest_id`, `user_id`, `team_id`, `status`, `registered_at`) values (?, ?, ?, ?, ?) " +
		"on duplicate key update `team_id`=values(`team_id`), `status`=values(`status`), `registered_at`=values(`registered_at`)"
	_, err := m.conn.ExecCtx(ctx, query, row.ContestId, row.UserId, row.TeamId, row.Status, row.RegisteredAt)
	return err
}

func (m *ContestParticipantModel) FindOne(ctx context.Context, contestID string, userID int64) (ContestParticipant, error) {
	query := "select `contest_id`, `user_id`, `team_id`, `status`, `registered_at` from " + m.table + " where `contest_id`=? and `user_id`=? limit 1"
	var resp ContestParticipant
	err := m.conn.QueryRowCtx(ctx, &resp, query, contestID, userID)
	return resp, err
}

func (m *ContestParticipantModel) ListByContest(ctx context.Context, contestID string, limit, offset int) ([]ContestParticipant, error) {
	if limit <= 0 {
		limit = 20
	}
	query := "select `contest_id`, `user_id`, `team_id`, `status`, `registered_at` from " + m.table + " where `contest_id`=? " +
		"order by `registered_at` asc, `user_id` asc limit ? offset ?"
	var rows []ContestParticipant
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, contestID, limit, offset); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return rows, nil
}

func (m *ContestParticipantModel) CountByContest(ctx context.Context, contestID string) (int, error) {
	query := "select count(1) from " + m.table + " where `contest_id`=?"
	var total int
	if err := m.conn.QueryRowCtx(ctx, &total, query, contestID); err != nil {
		return 0, err
	}
	return total, nil
}
