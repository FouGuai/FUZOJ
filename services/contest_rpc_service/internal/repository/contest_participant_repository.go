package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/cache_helper"
	"fuzoj/services/contest_rpc_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	ErrParticipantNotFound = errors.New("participant not found")
)

type ContestParticipant struct {
	ContestID    string
	UserID       int64
	Status       string
	RegisteredAt time.Time
}

type ContestParticipantRepository interface {
	GetParticipant(ctx context.Context, contestID string, userID int64) (ContestParticipant, error)
	InvalidateParticipantCache(ctx context.Context, contestID string, userID int64) error
}

type MySQLContestParticipantRepository struct {
	model    *model.ContestParticipantModel
	cache    cache.Cache
	local    *localCache[ContestParticipant]
	ttl      time.Duration
	emptyTTL time.Duration
}

func NewContestParticipantRepository(conn sqlx.SqlConn, cacheClient cache.Cache, ttl, emptyTTL time.Duration, localSize int, localTTL time.Duration) ContestParticipantRepository {
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
	return &MySQLContestParticipantRepository{
		model:    model.NewContestParticipantModel(conn),
		cache:    cacheClient,
		local:    newLocalCache[ContestParticipant](localSize, localTTL),
		ttl:      ttl,
		emptyTTL: emptyTTL,
	}
}

func (r *MySQLContestParticipantRepository) GetParticipant(ctx context.Context, contestID string, userID int64) (ContestParticipant, error) {
	if contestID == "" || userID <= 0 {
		return ContestParticipant{}, errors.New("contestID and userID are required")
	}
	key := contestParticipantKey(contestID, userID)
	if r.local != nil {
		if cached, ok := r.local.Get(key); ok {
			if cached.ContestID == "" {
				return ContestParticipant{}, ErrParticipantNotFound
			}
			return cached, nil
		}
	}
	if r.cache != nil {
		var cached ContestParticipant
		if err := r.cache.GetCtx(ctx, key, &cached); err == nil {
			if cached.ContestID == "" {
				return ContestParticipant{}, ErrParticipantNotFound
			}
			if r.local != nil {
				r.local.Set(key, cached)
			}
			return cached, nil
		} else if !r.cache.IsNotFound(err) {
			return ContestParticipant{}, err
		}
	}
	row, err := r.model.FindOne(ctx, contestID, userID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			if r.cache != nil {
				_ = r.cache.SetWithExpireCtx(ctx, key, ContestParticipant{}, cache_helper.JitterTTL(r.emptyTTL))
			}
			return ContestParticipant{}, ErrParticipantNotFound
		}
		return ContestParticipant{}, err
	}
	participant := ContestParticipant{
		ContestID:    row.ContestId,
		UserID:       row.UserId,
		Status:       row.Status,
		RegisteredAt: row.RegisteredAt,
	}
	if r.cache != nil {
		_ = r.cache.SetWithExpireCtx(ctx, key, participant, cache_helper.JitterTTL(r.ttl))
	}
	if r.local != nil {
		r.local.Set(key, participant)
	}
	return participant, nil
}

func (r *MySQLContestParticipantRepository) InvalidateParticipantCache(ctx context.Context, contestID string, userID int64) error {
	if contestID == "" || userID <= 0 {
		return errors.New("contestID and userID are required")
	}
	key := contestParticipantKey(contestID, userID)
	if r.local != nil {
		r.local.Delete(key)
	}
	if r.cache == nil {
		return nil
	}
	return r.cache.DelCtx(ctx, key)
}

func contestParticipantKey(contestID string, userID int64) string {
	return fmt.Sprintf("contest:participant:%s:%d", contestID, userID)
}
