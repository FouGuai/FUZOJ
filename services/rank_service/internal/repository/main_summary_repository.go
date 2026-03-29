package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const contestMemberSummaryTable = "`contest_member_summary_snapshot`"

// MainMemberSummary stores contest member summary rows from contest main table.
type MainMemberSummary struct {
	ContestID    string
	MemberID     string
	ScoreTotal   int64
	PenaltyTotal int64
	ACCount      int64
	DetailJSON   string
	Version      int64
	UpdatedAt    time.Time
}

// MainSummaryRepository loads contest member summary rows for rank recovery.
type MainSummaryRepository struct {
	conn sqlRunner
}

func NewMainSummaryRepository(conn sqlRunner) *MainSummaryRepository {
	return &MainSummaryRepository{conn: conn}
}

func (r *MainSummaryRepository) ListByContestAfterMember(ctx context.Context, contestID, lastMemberID string, limit int) ([]MainMemberSummary, error) {
	if r == nil || r.conn == nil {
		return nil, errors.New("main summary repository is not configured")
	}
	if contestID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 500
	}
	var rows []struct {
		ContestID    string         `db:"contest_id"`
		MemberID     string         `db:"member_id"`
		ScoreTotal   int64          `db:"score_total"`
		PenaltyTotal int64          `db:"penalty_total"`
		ACCount      int64          `db:"ac_count"`
		DetailJSON   sql.NullString `db:"detail_json"`
		Version      int64          `db:"version"`
		UpdatedAt    time.Time      `db:"updated_at"`
	}
	query := "select contest_id, member_id, score_total, penalty_total, ac_count, detail_json, version, updated_at " +
		"from " + contestMemberSummaryTable + " where contest_id = ? and member_id > ? " +
		"order by member_id asc limit ?"
	if err := r.conn.QueryRowsCtx(ctx, &rows, query, contestID, lastMemberID, limit); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	out := make([]MainMemberSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, MainMemberSummary{
			ContestID:    row.ContestID,
			MemberID:     row.MemberID,
			ScoreTotal:   row.ScoreTotal,
			PenaltyTotal: row.PenaltyTotal,
			ACCount:      row.ACCount,
			DetailJSON:   row.DetailJSON.String,
			Version:      row.Version,
			UpdatedAt:    row.UpdatedAt,
		})
	}
	return out, nil
}
