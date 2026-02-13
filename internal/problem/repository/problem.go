package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
)

const (
	defaultProblemLatestTTL      = 30 * time.Minute
	defaultProblemLatestEmptyTTL = 5 * time.Minute
	problemLatestKeyPrefix       = "problem:latest:"
)

const (
	problemVersionStatePublished = 1
)

var (
	ErrProblemNotFound = errors.New("problem not found")
)

// ProblemLatestMeta represents latest published meta for a problem.
type ProblemLatestMeta struct {
	ProblemID    int64
	Version      int32
	ManifestHash string
	DataPackKey  string
	DataPackHash string
	UpdatedAt    time.Time
}

type ProblemRepository interface {
	Create(ctx context.Context, tx db.Transaction, problem *Problem) (int64, error)
	Delete(ctx context.Context, tx db.Transaction, problemID int64) error
	GetLatestMeta(ctx context.Context, tx db.Transaction, problemID int64) (ProblemLatestMeta, error)
}

type MySQLProblemRepository struct {
	db       db.Database
	cache    cache.Cache
	ttl      time.Duration
	emptyTTL time.Duration
}

func NewProblemRepository(database db.Database, cacheClient cache.Cache) ProblemRepository {
	return NewProblemRepositoryWithTTL(database, cacheClient, defaultProblemLatestTTL, defaultProblemLatestEmptyTTL)
}

func NewProblemRepositoryWithTTL(database db.Database, cacheClient cache.Cache, ttl, emptyTTL time.Duration) ProblemRepository {
	if ttl <= 0 {
		ttl = defaultProblemLatestTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultProblemLatestEmptyTTL
	}
	return &MySQLProblemRepository{
		db:       database,
		cache:    cacheClient,
		ttl:      ttl,
		emptyTTL: emptyTTL,
	}
}

func (r *MySQLProblemRepository) GetLatestMeta(ctx context.Context, tx db.Transaction, problemID int64) (ProblemLatestMeta, error) {
	if r.cache != nil && tx == nil {
		meta, err := cache.GetWithCached[ProblemLatestMeta](
			ctx,
			r.cache,
			problemLatestKey(problemID),
			cache.JitterTTL(r.ttl),
			cache.JitterTTL(r.emptyTTL),
			func(meta ProblemLatestMeta) bool { return meta.ProblemID == 0 },
			marshalProblemLatestMeta,
			unmarshalProblemLatestMeta,
			func(ctx context.Context) (ProblemLatestMeta, error) {
				meta, err := r.getLatestMetaFromDB(ctx, nil, problemID)
				if err != nil {
					if errors.Is(err, ErrProblemNotFound) {
						return ProblemLatestMeta{}, nil
					}
					return ProblemLatestMeta{}, err
				}
				return meta, nil
			},
		)
		if err != nil {
			return ProblemLatestMeta{}, err
		}
		if meta.ProblemID == 0 {
			return ProblemLatestMeta{}, ErrProblemNotFound
		}
		return meta, nil
	}
	return r.getLatestMetaFromDB(ctx, tx, problemID)
}

func (r *MySQLProblemRepository) Create(ctx context.Context, tx db.Transaction, problem *Problem) (int64, error) {
	if problem == nil {
		return 0, errors.New("problem is nil")
	}
	if problem.Status == 0 {
		problem.Status = ProblemStatusDraft
	}

	query := "INSERT INTO problem (title, status, owner_id) VALUES (?, ?, ?)"
	result, err := db.GetQuerier(r.db, tx).Exec(ctx, query, problem.Title, problem.Status, problem.OwnerID)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	problem.ID = id
	return id, nil
}

func (r *MySQLProblemRepository) Delete(ctx context.Context, tx db.Transaction, problemID int64) error {
	query := "DELETE FROM problem WHERE id = ?"
	result, err := db.GetQuerier(r.db, tx).Exec(ctx, query, problemID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrProblemNotFound
	}
	return nil
}

func (r *MySQLProblemRepository) getLatestMetaFromDB(ctx context.Context, tx db.Transaction, problemID int64) (ProblemLatestMeta, error) {
	query := `
		SELECT problem_id, version, manifest_hash, data_pack_key, data_pack_hash, created_at
		FROM problem_version
		WHERE problem_id = ? AND state = ?
		ORDER BY version DESC
		LIMIT 1`

	row := db.GetQuerier(r.db, tx).QueryRow(ctx, query, problemID, problemVersionStatePublished)
	meta, err := scanProblemLatestMeta(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ProblemLatestMeta{}, ErrProblemNotFound
		}
		return ProblemLatestMeta{}, err
	}
	return meta, nil
}

func problemLatestKey(problemID int64) string {
	return problemLatestKeyPrefix + fmtInt64(problemID)
}

func marshalProblemLatestMeta(meta ProblemLatestMeta) string {
	payload, err := json.Marshal(meta)
	if err != nil {
		return ""
	}
	return string(payload)
}

func unmarshalProblemLatestMeta(data string) (ProblemLatestMeta, error) {
	if data == "" {
		return ProblemLatestMeta{}, nil
	}
	var meta ProblemLatestMeta
	if err := json.Unmarshal([]byte(data), &meta); err != nil {
		return ProblemLatestMeta{}, err
	}
	return meta, nil
}

func scanProblemLatestMeta(scanner db.Scanner) (ProblemLatestMeta, error) {
	var meta ProblemLatestMeta
	err := scanner.Scan(
		&meta.ProblemID,
		&meta.Version,
		&meta.ManifestHash,
		&meta.DataPackKey,
		&meta.DataPackHash,
		&meta.UpdatedAt,
	)
	if err != nil {
		return ProblemLatestMeta{}, err
	}
	return meta, nil
}

func fmtInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
