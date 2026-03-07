package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Contest struct {
	ContestId   string    `db:"contest_id"`
	Title       string    `db:"title"`
	Description string    `db:"description"`
	Status      string    `db:"status"`
	Visibility  string    `db:"visibility"`
	OwnerId     int64     `db:"owner_id"`
	OrgId       int64     `db:"org_id"`
	StartAt     time.Time `db:"start_at"`
	EndAt       time.Time `db:"end_at"`
	RuleJSON    string    `db:"rule_json"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
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

func (m *ContestModel) Insert(ctx context.Context, contest Contest) error {
	query := "insert into " + m.table + " (contest_id, title, description, status, visibility, owner_id, org_id, start_at, end_at, rule_json, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := m.conn.ExecCtx(ctx, query,
		contest.ContestId,
		contest.Title,
		contest.Description,
		contest.Status,
		contest.Visibility,
		contest.OwnerId,
		contest.OrgId,
		contest.StartAt,
		contest.EndAt,
		contest.RuleJSON,
		contest.CreatedAt,
		contest.UpdatedAt,
	)
	return err
}

func (m *ContestModel) FindByID(ctx context.Context, contestID string) (Contest, error) {
	var resp Contest
	query := "select contest_id, title, description, status, visibility, owner_id, org_id, start_at, end_at, rule_json, created_at, updated_at from " + m.table + " where contest_id = ? limit 1"
	if err := m.conn.QueryRowCtx(ctx, &resp, query, contestID); err != nil {
		return Contest{}, err
	}
	return resp, nil
}

func (m *ContestModel) List(ctx context.Context, where string, args []any, limit, offset int) ([]Contest, error) {
	if limit <= 0 {
		limit = 20
	}
	query := "select contest_id, title, description, status, visibility, owner_id, org_id, start_at, end_at, rule_json, created_at, updated_at from " + m.table
	if where != "" {
		query += " where " + where
	}
	query += " order by start_at desc, created_at desc limit ? offset ?"
	args = append(args, limit, offset)

	var resp []Contest
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return resp, nil
}

func (m *ContestModel) Count(ctx context.Context, where string, args []any) (int, error) {
	query := "select count(1) as total from " + m.table
	if where != "" {
		query += " where " + where
	}
	var total int
	if err := m.conn.QueryRowCtx(ctx, &total, query, args...); err != nil {
		return 0, err
	}
	return total, nil
}

func (m *ContestModel) Update(ctx context.Context, setClause string, args []any) (int64, error) {
	query := "update " + m.table + " set " + setClause
	res, err := m.conn.ExecCtx(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}
