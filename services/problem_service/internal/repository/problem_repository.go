package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"fuzoj/services/problem_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	defaultProblemLatestTTL      = 30 * time.Minute
	defaultProblemLatestEmptyTTL = 5 * time.Minute
	problemLatestKeyPrefix       = "problem:latest:"
)

var (
	ErrProblemNotFound = errors.New("problem not found")
)

// ProblemRepository provides problem meta access.
type ProblemRepository interface {
	Create(ctx context.Context, session sqlx.Session, problem *Problem) (int64, error)
	Delete(ctx context.Context, session sqlx.Session, problemID int64) error
	Exists(ctx context.Context, session sqlx.Session, problemID int64) (bool, error)
	GetLatestMeta(ctx context.Context, session sqlx.Session, problemID int64) (ProblemLatestMeta, error)
	InvalidateLatestMetaCache(ctx context.Context, problemID int64) error
}

type MySQLProblemRepository struct {
	problemModel model.ProblemModel
	versionModel model.ProblemVersionModel
	cache        cache.Cache
	ttl          time.Duration
	emptyTTL     time.Duration
}

func NewProblemRepository(conn sqlx.SqlConn, cacheClient cache.Cache) ProblemRepository {
	return NewProblemRepositoryWithTTL(conn, cacheClient, defaultProblemLatestTTL, defaultProblemLatestEmptyTTL)
}

func NewProblemRepositoryWithTTL(conn sqlx.SqlConn, cacheClient cache.Cache, ttl, emptyTTL time.Duration) ProblemRepository {
	if ttl <= 0 {
		ttl = defaultProblemLatestTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultProblemLatestEmptyTTL
	}
	return &MySQLProblemRepository{
		problemModel: model.NewProblemModel(conn),
		versionModel: model.NewProblemVersionModel(conn),
		cache:        cacheClient,
		ttl:          ttl,
		emptyTTL:     emptyTTL,
	}
}

func (r *MySQLProblemRepository) GetLatestMeta(ctx context.Context, session sqlx.Session, problemID int64) (ProblemLatestMeta, error) {
	if r.cache != nil && session == nil {
		key := problemLatestKey(problemID)
		var cached ProblemLatestMeta
		if err := r.cache.GetCtx(ctx, key, &cached); err == nil {
			if cached.ProblemID == 0 {
				return ProblemLatestMeta{}, ErrProblemNotFound
			}
			return cached, nil
		} else if !r.cache.IsNotFound(err) {
			return ProblemLatestMeta{}, err
		}

		meta, err := r.getLatestMetaFromDB(ctx, nil, problemID)
		if err != nil {
			if errors.Is(err, ErrProblemNotFound) {
				_ = r.cache.SetWithExpireCtx(ctx, key, ProblemLatestMeta{}, jitterTTL(r.emptyTTL))
				return ProblemLatestMeta{}, ErrProblemNotFound
			}
			return ProblemLatestMeta{}, err
		}
		_ = r.cache.SetWithExpireCtx(ctx, key, meta, jitterTTL(r.ttl))
		return meta, nil
	}
	return r.getLatestMetaFromDB(ctx, session, problemID)
}

func (r *MySQLProblemRepository) Exists(ctx context.Context, session sqlx.Session, problemID int64) (bool, error) {
	return r.problemModel.WithSession(session).Exists(ctx, problemID)
}

func (r *MySQLProblemRepository) InvalidateLatestMetaCache(ctx context.Context, problemID int64) error {
	if r.cache == nil {
		return nil
	}
	if problemID <= 0 {
		return errors.New("problemID is required")
	}
	return r.cache.DelCtx(ctx, problemLatestKey(problemID))
}

func (r *MySQLProblemRepository) Create(ctx context.Context, session sqlx.Session, problem *Problem) (int64, error) {
	if problem == nil {
		return 0, errors.New("problem is nil")
	}
	if problem.Status == 0 {
		problem.Status = ProblemStatusDraft
	}

	result, err := r.problemModel.WithSession(session).Insert(ctx, &model.Problem{
		Title:   problem.Title,
		Status:  int64(problem.Status),
		OwnerId: problem.OwnerID,
	})
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

func (r *MySQLProblemRepository) Delete(ctx context.Context, session sqlx.Session, problemID int64) error {
	deleted, err := r.problemModel.WithSession(session).DeleteByID(ctx, problemID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrProblemNotFound
	}
	return nil
}

func (r *MySQLProblemRepository) getLatestMetaFromDB(ctx context.Context, session sqlx.Session, problemID int64) (ProblemLatestMeta, error) {
	row, err := r.versionModel.WithSession(session).FindLatestPublishedByProblemID(ctx, problemID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ProblemLatestMeta{}, ErrProblemNotFound
		}
		return ProblemLatestMeta{}, err
	}
	return ProblemLatestMeta{
		ProblemID:    row.ProblemId,
		Version:      int32(row.Version),
		ManifestHash: row.ManifestHash,
		DataPackKey:  row.DataPackKey,
		DataPackHash: row.DataPackHash,
		UpdatedAt:    row.CreatedAt,
	}, nil
}

func problemLatestKey(problemID int64) string {
	return problemLatestKeyPrefix + strconv.FormatInt(problemID, 10)
}
