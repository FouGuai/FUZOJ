package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const contestMemberSummaryTable = "`contest_member_summary_snapshot`"

// MemberSummarySnapshot stores member summary snapshot.
type MemberSummarySnapshot struct {
	ContestID    string
	MemberID     string
	ScoreTotal   int64
	PenaltyTotal int64
	ACCount      int64
	DetailJSON   string
	Version      int64
	UpdatedAt    time.Time
}

// MemberSummaryRepository handles contest member summary persistence.
type MemberSummaryRepository struct {
	conn sqlRunner
}

func NewMemberSummaryRepository(conn sqlRunner) *MemberSummaryRepository {
	return &MemberSummaryRepository{conn: conn}
}

func (r *MemberSummaryRepository) Get(ctx context.Context, contestID, memberID string) (MemberSummarySnapshot, bool, error) {
	if r == nil || r.conn == nil {
		return MemberSummarySnapshot{}, false, errors.New("member summary repository is not configured")
	}
	var resp struct {
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
		"from " + contestMemberSummaryTable + " where contest_id = ? and member_id = ? limit 1"
	if err := r.conn.QueryRowCtx(ctx, &resp, query, contestID, memberID); err != nil {
		if err == sqlx.ErrNotFound {
			return MemberSummarySnapshot{}, false, nil
		}
		return MemberSummarySnapshot{}, false, err
	}
	snapshot := MemberSummarySnapshot{
		ContestID:    resp.ContestID,
		MemberID:     resp.MemberID,
		ScoreTotal:   resp.ScoreTotal,
		PenaltyTotal: resp.PenaltyTotal,
		ACCount:      resp.ACCount,
		DetailJSON:   resp.DetailJSON.String,
		Version:      resp.Version,
		UpdatedAt:    resp.UpdatedAt,
	}
	return snapshot, true, nil
}

func (r *MemberSummaryRepository) Upsert(ctx context.Context, snapshot MemberSummarySnapshot) error {
	if r == nil || r.conn == nil {
		return errors.New("member summary repository is not configured")
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now()
	}
	query := "insert into " + contestMemberSummaryTable + " (contest_id, member_id, score_total, penalty_total, ac_count, detail_json, version, updated_at) " +
		"values (?, ?, ?, ?, ?, ?, ?, ?) " +
		"on duplicate key update score_total=values(score_total), penalty_total=values(penalty_total), ac_count=values(ac_count), detail_json=values(detail_json), version=values(version), updated_at=values(updated_at)"
	_, err := r.conn.ExecCtx(ctx, query,
		snapshot.ContestID,
		snapshot.MemberID,
		snapshot.ScoreTotal,
		snapshot.PenaltyTotal,
		snapshot.ACCount,
		nullString(snapshot.DetailJSON),
		snapshot.Version,
		snapshot.UpdatedAt,
	)
	return err
}
