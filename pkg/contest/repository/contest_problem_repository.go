package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/cache_helper"
	"fuzoj/pkg/contest/model"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	ErrContestProblemNotFound = errors.New("contest problem not found")
)

type ContestProblemRepository interface {
	HasProblem(ctx context.Context, contestID string, problemID int64) (bool, error)
	InvalidateProblemCache(ctx context.Context, contestID string, problemID int64) error
}

type MySQLContestProblemRepository struct {
	model    *model.ContestProblemModel
	cache    cache.Cache
	local    *localCache[bool]
	ttl      time.Duration
	emptyTTL time.Duration
}

func NewContestProblemRepository(conn sqlx.SqlConn, cacheClient cache.Cache, ttl, emptyTTL time.Duration, localSize int, localTTL time.Duration) ContestProblemRepository {
	if ttl <= 0 {
		ttl = defaultEligibilityTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultEligibilityEmptyTTL
	}
	if localSize <= 0 {
		localSize = defaultEligibilityLocalSize
	}
	if localTTL <= 0 {
		localTTL = ttl
	}
	return &MySQLContestProblemRepository{
		model:    model.NewContestProblemModel(conn),
		cache:    cacheClient,
		local:    newLocalCache[bool](localSize, localTTL),
		ttl:      ttl,
		emptyTTL: emptyTTL,
	}
}

func (r *MySQLContestProblemRepository) HasProblem(ctx context.Context, contestID string, problemID int64) (bool, error) {
	if contestID == "" || problemID <= 0 {
		return false, errors.New("contestID and problemID are required")
	}
	key := contestProblemKey(contestID, problemID)
	if r.local != nil {
		if cached, ok := r.local.Get(key); ok {
			return cached, nil
		}
	}
	if r.cache != nil {
		var cached bool
		if err := r.cache.GetCtx(ctx, key, &cached); err == nil {
			if r.local != nil {
				r.local.Set(key, cached)
			}
			return cached, nil
		} else if !r.cache.IsNotFound(err) {
			return false, err
		}
	}
	exists, err := r.model.Exists(ctx, contestID, problemID)
	if err != nil {
		return false, err
	}
	if r.cache != nil {
		ttl := r.ttl
		if !exists {
			ttl = r.emptyTTL
		}
		_ = r.cache.SetWithExpireCtx(ctx, key, exists, cache_helper.JitterTTL(ttl))
	}
	if r.local != nil {
		r.local.Set(key, exists)
	}
	if !exists {
		return false, ErrContestProblemNotFound
	}
	return true, nil
}

func (r *MySQLContestProblemRepository) InvalidateProblemCache(ctx context.Context, contestID string, problemID int64) error {
	if contestID == "" || problemID <= 0 {
		return errors.New("contestID and problemID are required")
	}
	key := contestProblemKey(contestID, problemID)
	if r.local != nil {
		r.local.Delete(key)
	}
	if r.cache == nil {
		return nil
	}
	return r.cache.DelCtx(ctx, key)
}

func contestProblemKey(contestID string, problemID int64) string {
	return fmt.Sprintf("contest:problem:%s:%d", contestID, problemID)
}
