package repository

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	rankSnapshotMetaTable  = "`rank_snapshot_meta`"
	rankSnapshotEntryTable = "`rank_snapshot_entry`"
)

// SnapshotMeta stores snapshot metadata.
type SnapshotMeta struct {
	ID           int64
	ContestID    string
	SnapshotAt   time.Time
	LastResultID int64
	LastVersion  int64
	Total        int64
	Status       string
}

// SnapshotEntry stores a rank snapshot entry.
type SnapshotEntry struct {
	SnapshotID  int64
	MemberID    string
	Rank        int64
	SortScore   int64
	ScoreTotal  int64
	Penalty     int64
	ACCount     int64
	DetailJSON  string
	SummaryJSON string
}

// SnapshotRepository handles snapshot persistence.
type SnapshotRepository struct {
	conn sqlRunner
}

func NewSnapshotRepository(conn sqlRunner) *SnapshotRepository {
	return &SnapshotRepository{conn: conn}
}

func (r *SnapshotRepository) CreateSnapshotMeta(ctx context.Context, meta SnapshotMeta) (int64, error) {
	if r == nil || r.conn == nil {
		return 0, errors.New("snapshot repository is not configured")
	}
	if meta.SnapshotAt.IsZero() {
		meta.SnapshotAt = time.Now()
	}
	if meta.Status == "" {
		meta.Status = "writing"
	}
	query := "insert into " + rankSnapshotMetaTable +
		" (contest_id, snapshot_at, last_result_id, last_version, total, status, created_at, updated_at) " +
		"values (?, ?, ?, ?, ?, ?, ?, ?)"
	now := time.Now()
	res, err := r.conn.ExecCtx(ctx, query,
		meta.ContestID,
		meta.SnapshotAt,
		meta.LastResultID,
		meta.LastVersion,
		meta.Total,
		meta.Status,
		now,
		now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *SnapshotRepository) MarkSnapshotReady(ctx context.Context, id int64) error {
	if r == nil || r.conn == nil {
		return errors.New("snapshot repository is not configured")
	}
	if id <= 0 {
		return nil
	}
	query := "update " + rankSnapshotMetaTable + " set status = 'ready', updated_at = ? where id = ?"
	_, err := r.conn.ExecCtx(ctx, query, time.Now(), id)
	return err
}

func (r *SnapshotRepository) InsertSnapshotEntries(ctx context.Context, entries []SnapshotEntry) error {
	if r == nil || r.conn == nil {
		return errors.New("snapshot repository is not configured")
	}
	if len(entries) == 0 {
		return nil
	}
	query := "insert into " + rankSnapshotEntryTable +
		" (snapshot_id, member_id, `rank`, sort_score, score_total, penalty_total, ac_count, detail_json, summary_json) values "
	args := make([]any, 0, len(entries)*9)
	for i, entry := range entries {
		if i > 0 {
			query += ","
		}
		query += "(?, ?, ?, ?, ?, ?, ?, ?, ?)"
		args = append(args,
			entry.SnapshotID,
			entry.MemberID,
			entry.Rank,
			entry.SortScore,
			entry.ScoreTotal,
			entry.Penalty,
			entry.ACCount,
			nullString(entry.DetailJSON),
			entry.SummaryJSON,
		)
	}
	_, err := r.conn.ExecCtx(ctx, query, args...)
	return err
}

func (r *SnapshotRepository) LoadLatestReadySnapshotMeta(ctx context.Context, contestID string) (SnapshotMeta, bool, error) {
	if r == nil || r.conn == nil {
		return SnapshotMeta{}, false, errors.New("snapshot repository is not configured")
	}
	if contestID == "" {
		return SnapshotMeta{}, false, nil
	}
	var resp struct {
		ID           int64     `db:"id"`
		ContestID    string    `db:"contest_id"`
		SnapshotAt   time.Time `db:"snapshot_at"`
		LastResultID int64     `db:"last_result_id"`
		LastVersion  int64     `db:"last_version"`
		Total        int64     `db:"total"`
		Status       string    `db:"status"`
	}
	query := "select id, contest_id, snapshot_at, last_result_id, last_version, total, status " +
		"from " + rankSnapshotMetaTable + " where contest_id = ? and status = 'ready' " +
		"order by snapshot_at desc limit 1"
	if err := r.conn.QueryRowCtx(ctx, &resp, query, contestID); err != nil {
		if err == sqlx.ErrNotFound {
			return SnapshotMeta{}, false, nil
		}
		return SnapshotMeta{}, false, err
	}
	return SnapshotMeta{
		ID:           resp.ID,
		ContestID:    resp.ContestID,
		SnapshotAt:   resp.SnapshotAt,
		LastResultID: resp.LastResultID,
		LastVersion:  resp.LastVersion,
		Total:        resp.Total,
		Status:       resp.Status,
	}, true, nil
}

func (r *SnapshotRepository) ListLatestReadySnapshotMetas(ctx context.Context) ([]SnapshotMeta, error) {
	if r == nil || r.conn == nil {
		return nil, errors.New("snapshot repository is not configured")
	}
	var resp []SnapshotMeta
	query := "select m.id, m.contest_id, m.snapshot_at, m.last_result_id, m.last_version, m.total, m.status " +
		"from " + rankSnapshotMetaTable + " m " +
		"join (" +
		"  select contest_id, max(snapshot_at) as snapshot_at " +
		"  from " + rankSnapshotMetaTable + " where status = 'ready' group by contest_id" +
		") t on m.contest_id = t.contest_id and m.snapshot_at = t.snapshot_at " +
		"where m.status = 'ready'"
	if err := r.conn.QueryRowsCtx(ctx, &resp, query); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return resp, nil
}

func (r *SnapshotRepository) ListSnapshotEntriesAfterRank(ctx context.Context, snapshotID, lastRank int64, limit int) ([]SnapshotEntry, error) {
	if r == nil || r.conn == nil {
		return nil, errors.New("snapshot repository is not configured")
	}
	if snapshotID <= 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	var resp []SnapshotEntry
	query := "select snapshot_id, member_id, `rank`, sort_score, score_total, penalty_total, ac_count, detail_json, summary_json " +
		"from " + rankSnapshotEntryTable + " where snapshot_id = ? and `rank` > ? " +
		"order by `rank` asc limit ?"
	if err := r.conn.QueryRowsCtx(ctx, &resp, query, snapshotID, lastRank, limit); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return resp, nil
}
