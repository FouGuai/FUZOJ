package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const contestMemberProblemTable = "`contest_member_problem_state`"

// MemberProblemState stores per-member per-problem contest state.
type MemberProblemState struct {
	ContestID        string
	MemberID         string
	ProblemID        int64
	Solved           bool
	FirstACAt        time.Time
	WrongCount       int
	Score            int
	Penalty          int64
	LastSubmissionID string
	LastSubmissionAt time.Time
	UpdatedAt        time.Time
}

// MemberProblemRepository handles contest member-problem persistence.
type MemberProblemRepository struct {
	conn sqlRunner
}

func NewMemberProblemRepository(conn sqlRunner) *MemberProblemRepository {
	return &MemberProblemRepository{conn: conn}
}

func (r *MemberProblemRepository) Get(ctx context.Context, contestID, memberID string, problemID int64) (MemberProblemState, bool, error) {
	if r == nil || r.conn == nil {
		return MemberProblemState{}, false, errors.New("member problem repository is not configured")
	}
	var resp struct {
		ContestID        string         `db:"contest_id"`
		MemberID         string         `db:"member_id"`
		ProblemID        int64          `db:"problem_id"`
		Solved           int            `db:"solved"`
		FirstACAt        sql.NullTime   `db:"first_ac_at"`
		WrongCount       int            `db:"wrong_count"`
		Score            int            `db:"score"`
		Penalty          int64          `db:"penalty"`
		LastSubmissionID sql.NullString `db:"last_submission_id"`
		LastSubmissionAt sql.NullTime   `db:"last_submission_at"`
		UpdatedAt        time.Time      `db:"updated_at"`
	}
	query := "select contest_id, member_id, problem_id, solved, first_ac_at, wrong_count, score, penalty, last_submission_id, last_submission_at, updated_at " +
		"from " + contestMemberProblemTable + " where contest_id = ? and member_id = ? and problem_id = ? limit 1"
	if err := r.conn.QueryRowCtx(ctx, &resp, query, contestID, memberID, problemID); err != nil {
		if err == sqlx.ErrNotFound {
			return MemberProblemState{}, false, nil
		}
		return MemberProblemState{}, false, err
	}
	state := MemberProblemState{
		ContestID:        resp.ContestID,
		MemberID:         resp.MemberID,
		ProblemID:        resp.ProblemID,
		Solved:           resp.Solved == 1,
		WrongCount:       resp.WrongCount,
		Score:            resp.Score,
		Penalty:          resp.Penalty,
		LastSubmissionID: resp.LastSubmissionID.String,
		UpdatedAt:        resp.UpdatedAt,
	}
	if resp.FirstACAt.Valid {
		state.FirstACAt = resp.FirstACAt.Time
	}
	if resp.LastSubmissionAt.Valid {
		state.LastSubmissionAt = resp.LastSubmissionAt.Time
	}
	return state, true, nil
}

func (r *MemberProblemRepository) Upsert(ctx context.Context, state MemberProblemState) error {
	if r == nil || r.conn == nil {
		return errors.New("member problem repository is not configured")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now()
	}
	solved := 0
	if state.Solved {
		solved = 1
	}
	query := "insert into " + contestMemberProblemTable + " (contest_id, member_id, problem_id, solved, first_ac_at, wrong_count, score, penalty, last_submission_id, last_submission_at, updated_at) " +
		"values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) " +
		"on duplicate key update solved=values(solved), first_ac_at=values(first_ac_at), wrong_count=values(wrong_count), score=values(score), penalty=values(penalty), " +
		"last_submission_id=values(last_submission_id), last_submission_at=values(last_submission_at), updated_at=values(updated_at)"
	_, err := r.conn.ExecCtx(ctx, query,
		state.ContestID,
		state.MemberID,
		state.ProblemID,
		solved,
		nullTime(state.FirstACAt),
		state.WrongCount,
		state.Score,
		state.Penalty,
		nullString(state.LastSubmissionID),
		nullTime(state.LastSubmissionAt),
		state.UpdatedAt,
	)
	return err
}

func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
