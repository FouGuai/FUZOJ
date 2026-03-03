package repository

import (
	"context"
	"errors"
	"time"

	"fuzoj/internal/common/cache_helper"
	"fuzoj/pkg/contest/model"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	defaultEligibilityTTL       = 30 * time.Minute
	defaultEligibilityEmptyTTL  = 5 * time.Minute
	defaultEligibilityLocalSize = 1024
)

var (
	ErrContestNotFound = errors.New("contest not found")
)

type ContestMeta struct {
	ContestID  string
	Status     string
	Visibility string
	StartAt    time.Time
	EndAt      time.Time
}

type ContestRepository interface {
	GetMeta(ctx context.Context, contestID string) (ContestMeta, error)
	InvalidateMetaCache(ctx context.Context, contestID string) error
}

type MySQLContestRepository struct {
	model    *model.ContestModel
	cache    cache.Cache
	local    *localCache[ContestMeta]
	ttl      time.Duration
	emptyTTL time.Duration
}

func NewContestRepository(conn sqlx.SqlConn, cacheClient cache.Cache, ttl, emptyTTL time.Duration, localSize int, localTTL time.Duration) ContestRepository {
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
	return &MySQLContestRepository{
		model:    model.NewContestModel(conn),
		cache:    cacheClient,
		local:    newLocalCache[ContestMeta](localSize, localTTL),
		ttl:      ttl,
		emptyTTL: emptyTTL,
	}
}

func (r *MySQLContestRepository) GetMeta(ctx context.Context, contestID string) (ContestMeta, error) {
	if contestID == "" {
		return ContestMeta{}, errors.New("contestID is required")
	}
	key := contestMetaKey(contestID)
	if r.local != nil {
		if cached, ok := r.local.Get(key); ok {
			if cached.ContestID == "" {
				return ContestMeta{}, ErrContestNotFound
			}
			return cached, nil
		}
	}
	if r.cache != nil {
		var cached ContestMeta
		if err := r.cache.GetCtx(ctx, key, &cached); err == nil {
			if cached.ContestID == "" {
				return ContestMeta{}, ErrContestNotFound
			}
			if r.local != nil {
				r.local.Set(key, cached)
			}
			return cached, nil
		} else if !r.cache.IsNotFound(err) {
			return ContestMeta{}, err
		}
	}

	row, err := r.model.FindMeta(ctx, contestID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			if r.cache != nil {
				_ = r.cache.SetWithExpireCtx(ctx, key, ContestMeta{}, cache_helper.JitterTTL(r.emptyTTL))
			}
			return ContestMeta{}, ErrContestNotFound
		}
		return ContestMeta{}, err
	}
	meta := ContestMeta{
		ContestID:  row.ContestId,
		Status:     row.Status,
		Visibility: row.Visibility,
		StartAt:    row.StartAt,
		EndAt:      row.EndAt,
	}
	if r.cache != nil {
		_ = r.cache.SetWithExpireCtx(ctx, key, meta, cache_helper.JitterTTL(r.ttl))
	}
	if r.local != nil {
		r.local.Set(key, meta)
	}
	return meta, nil
}

func (r *MySQLContestRepository) InvalidateMetaCache(ctx context.Context, contestID string) error {
	if contestID == "" {
		return errors.New("contestID is required")
	}
	key := contestMetaKey(contestID)
	if r.local != nil {
		r.local.Delete(key)
	}
	if r.cache == nil {
		return nil
	}
	return r.cache.DelCtx(ctx, key)
}

func contestMetaKey(contestID string) string {
	return "contest:meta:" + contestID
}
