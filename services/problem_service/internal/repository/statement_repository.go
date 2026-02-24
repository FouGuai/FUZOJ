package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"fuzoj/internal/common/cache_helper"
	"fuzoj/services/problem_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	defaultStatementTTL       = 30 * time.Minute
	defaultStatementEmptyTTL  = 5 * time.Minute
	statementLatestKeyPrefix  = "problem:statement:latest:"
	statementVersionKeyPrefix = "problem:statement:version:"
)

var (
	ErrProblemStatementNotFound = errors.New("problem statement not found")
)

// ProblemStatementRepository provides problem statement access.
type ProblemStatementRepository interface {
	GetLatestPublished(ctx context.Context, session sqlx.Session, problemID int64) (ProblemStatement, error)
	GetByVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) (ProblemStatement, error)
	ExistsByVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error)
	Upsert(ctx context.Context, session sqlx.Session, statement ProblemStatement, problemVersionID int64) error
	InvalidateLatestCache(ctx context.Context, problemID int64) error
	InvalidateVersionCache(ctx context.Context, problemID int64, version int32) error
}

type MySQLProblemStatementRepository struct {
	statementModel model.ProblemStatementModel
	cache          cache.Cache
	local          *StatementLocalCache
	ttl            time.Duration
	emptyTTL       time.Duration
}

func NewProblemStatementRepository(conn sqlx.SqlConn, cacheClient cache.Cache, local *StatementLocalCache) ProblemStatementRepository {
	return NewProblemStatementRepositoryWithTTL(conn, cacheClient, local, defaultStatementTTL, defaultStatementEmptyTTL)
}

func NewProblemStatementRepositoryWithTTL(conn sqlx.SqlConn, cacheClient cache.Cache, local *StatementLocalCache, ttl, emptyTTL time.Duration) ProblemStatementRepository {
	if ttl <= 0 {
		ttl = defaultStatementTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultStatementEmptyTTL
	}
	return &MySQLProblemStatementRepository{
		statementModel: model.NewProblemStatementModel(conn),
		cache:          cacheClient,
		local:          local,
		ttl:            ttl,
		emptyTTL:       emptyTTL,
	}
}

func (r *MySQLProblemStatementRepository) GetLatestPublished(ctx context.Context, session sqlx.Session, problemID int64) (ProblemStatement, error) {
	if problemID <= 0 {
		return ProblemStatement{}, fmt.Errorf("problemID is required")
	}
	if session == nil {
		key := statementLatestKey(problemID)
		if r.local != nil {
			if cached, ok := r.local.Get(key); ok {
				return cached, nil
			}
		}
		if r.cache != nil {
			var cached ProblemStatement
			if err := r.cache.GetCtx(ctx, key, &cached); err == nil {
				if cached.ProblemID == 0 {
					return ProblemStatement{}, ErrProblemStatementNotFound
				}
				if r.local != nil {
					r.local.Set(key, cached, r.ttl)
				}
				return cached, nil
			} else if !r.cache.IsNotFound(err) {
				return ProblemStatement{}, err
			}
		}
	}

	statement, err := r.getLatestPublishedFromDB(ctx, session, problemID)
	if err != nil {
		if errors.Is(err, ErrProblemStatementNotFound) && session == nil && r.cache != nil {
			_ = r.cache.SetWithExpireCtx(ctx, statementLatestKey(problemID), ProblemStatement{}, cache_helper.JitterTTL(r.emptyTTL))
		}
		return ProblemStatement{}, err
	}
	if session == nil && r.cache != nil {
		_ = r.cache.SetWithExpireCtx(ctx, statementLatestKey(problemID), statement, cache_helper.JitterTTL(r.ttl))
	}
	if session == nil && r.local != nil {
		r.local.Set(statementLatestKey(problemID), statement, r.ttl)
	}
	return statement, nil
}

func (r *MySQLProblemStatementRepository) GetByVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) (ProblemStatement, error) {
	if problemID <= 0 || version <= 0 {
		return ProblemStatement{}, fmt.Errorf("problemID and version are required")
	}
	if session == nil {
		key := statementVersionKey(problemID, version)
		if r.local != nil {
			if cached, ok := r.local.Get(key); ok {
				return cached, nil
			}
		}
		if r.cache != nil {
			var cached ProblemStatement
			if err := r.cache.GetCtx(ctx, key, &cached); err == nil {
				if cached.ProblemID == 0 {
					return ProblemStatement{}, ErrProblemStatementNotFound
				}
				if r.local != nil {
					r.local.Set(key, cached, r.ttl)
				}
				return cached, nil
			} else if !r.cache.IsNotFound(err) {
				return ProblemStatement{}, err
			}
		}
	}
	statement, err := r.getByVersionFromDB(ctx, session, problemID, version)
	if err != nil {
		if errors.Is(err, ErrProblemStatementNotFound) && session == nil && r.cache != nil {
			_ = r.cache.SetWithExpireCtx(ctx, statementVersionKey(problemID, version), ProblemStatement{}, cache_helper.JitterTTL(r.emptyTTL))
		}
		return ProblemStatement{}, err
	}
	if session == nil && r.cache != nil {
		_ = r.cache.SetWithExpireCtx(ctx, statementVersionKey(problemID, version), statement, cache_helper.JitterTTL(r.ttl))
	}
	if session == nil && r.local != nil {
		r.local.Set(statementVersionKey(problemID, version), statement, r.ttl)
	}
	return statement, nil
}

func (r *MySQLProblemStatementRepository) ExistsByVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error) {
	if problemID <= 0 || version <= 0 {
		return false, fmt.Errorf("problemID and version are required")
	}
	_, err := r.statementModel.WithSession(session).FindByProblemIDVersion(ctx, problemID, int64(version))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, model.ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (r *MySQLProblemStatementRepository) Upsert(ctx context.Context, session sqlx.Session, statement ProblemStatement, problemVersionID int64) error {
	if statement.ProblemID <= 0 || statement.Version <= 0 {
		return fmt.Errorf("problemID and version are required")
	}
	if problemVersionID <= 0 {
		return fmt.Errorf("problemVersionID is required")
	}
	return r.statementModel.WithSession(session).Upsert(ctx, &model.ProblemStatement{
		ProblemVersionId: problemVersionID,
		ProblemId:        statement.ProblemID,
		Version:          int64(statement.Version),
		StatementMd:      statement.StatementMd,
		StatementHash:    statement.StatementHash,
	})
}

func (r *MySQLProblemStatementRepository) InvalidateLatestCache(ctx context.Context, problemID int64) error {
	if problemID <= 0 {
		return fmt.Errorf("problemID is required")
	}
	key := statementLatestKey(problemID)
	if r.local != nil {
		r.local.Delete(key)
	}
	if r.cache == nil {
		return nil
	}
	return r.cache.DelCtx(ctx, key)
}

func (r *MySQLProblemStatementRepository) InvalidateVersionCache(ctx context.Context, problemID int64, version int32) error {
	if problemID <= 0 || version <= 0 {
		return fmt.Errorf("problemID and version are required")
	}
	key := statementVersionKey(problemID, version)
	if r.local != nil {
		r.local.Delete(key)
	}
	if r.cache == nil {
		return nil
	}
	return r.cache.DelCtx(ctx, key)
}

func (r *MySQLProblemStatementRepository) getLatestPublishedFromDB(ctx context.Context, session sqlx.Session, problemID int64) (ProblemStatement, error) {
	row, err := r.statementModel.WithSession(session).FindLatestPublishedByProblemID(ctx, problemID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ProblemStatement{}, ErrProblemStatementNotFound
		}
		return ProblemStatement{}, err
	}
	return toProblemStatement(row), nil
}

func (r *MySQLProblemStatementRepository) getByVersionFromDB(ctx context.Context, session sqlx.Session, problemID int64, version int32) (ProblemStatement, error) {
	row, err := r.statementModel.WithSession(session).FindByProblemIDVersion(ctx, problemID, int64(version))
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ProblemStatement{}, ErrProblemStatementNotFound
		}
		return ProblemStatement{}, err
	}
	return toProblemStatement(row), nil
}

func toProblemStatement(row *model.ProblemStatement) ProblemStatement {
	if row == nil {
		return ProblemStatement{}
	}
	return ProblemStatement{
		ProblemID:     row.ProblemId,
		Version:       int32(row.Version),
		StatementMd:   row.StatementMd,
		StatementHash: row.StatementHash,
		UpdatedAt:     row.UpdatedAt,
	}
}

func statementLatestKey(problemID int64) string {
	return statementLatestKeyPrefix + strconv.FormatInt(problemID, 10)
}

func statementVersionKey(problemID int64, version int32) string {
	return statementVersionKeyPrefix + strconv.FormatInt(problemID, 10) + ":" + strconv.FormatInt(int64(version), 10)
}
