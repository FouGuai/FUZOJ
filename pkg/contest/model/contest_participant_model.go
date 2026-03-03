package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ContestParticipant struct {
	ContestId    string    `db:"contest_id"`
	UserId       int64     `db:"user_id"`
	Status       string    `db:"status"`
	RegisteredAt time.Time `db:"registered_at"`
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

func (m *ContestParticipantModel) FindOne(ctx context.Context, contestID string, userID int64) (ContestParticipant, error) {
	var resp ContestParticipant
	query := "select contest_id, user_id, status, registered_at from " + m.table + " where `contest_id` = ? and `user_id` = ? limit 1"
	err := m.conn.QueryRowCtx(ctx, &resp, query, contestID, userID)
	return resp, err
}
