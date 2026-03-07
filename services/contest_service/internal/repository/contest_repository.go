package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fuzoj/internal/common/cache_helper"
	"fuzoj/services/contest_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	ErrContestNotFound = errors.New("contest not found")
)

type ContestDetail struct {
	ContestID   string
	Title       string
	Description string
	Status      string
	Visibility  string
	OwnerID     int64
	OrgID       int64
	StartAt     time.Time
	EndAt       time.Time
	RuleJSON    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ContestListItem struct {
	ContestID string
	Title     string
	Status    string
	StartAt   time.Time
	EndAt     time.Time
	RuleJSON  string
}

type ContestListFilter struct {
	Status   string
	OwnerID  int64
	OrgID    int64
	Page     int
	PageSize int
}

type ContestCreateInput struct {
	ContestID   string
	Title       string
	Description string
	Status      string
	Visibility  string
	OwnerID     int64
	OrgID       int64
	StartAt     time.Time
	EndAt       time.Time
	RuleJSON    string
}

type ContestUpdate struct {
	Title       *string
	Description *string
	Visibility  *string
	StartAt     *time.Time
	EndAt       *time.Time
	RuleJSON    *string
}

type ContestRepository interface {
	Create(ctx context.Context, input ContestCreateInput) error
	Get(ctx context.Context, contestID string) (ContestDetail, error)
	List(ctx context.Context, filter ContestListFilter) ([]ContestListItem, int, error)
	Update(ctx context.Context, contestID string, update ContestUpdate) error
	InvalidateDetailCache(ctx context.Context, contestID string) error
}

type MySQLContestRepository struct {
	model    *model.ContestModel
	cache    cache.Cache
	ttl      time.Duration
	emptyTTL time.Duration
}

func NewContestRepository(conn sqlx.SqlConn, cacheClient cache.Cache, ttl, emptyTTL time.Duration) ContestRepository {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if emptyTTL <= 0 {
		emptyTTL = 5 * time.Minute
	}
	return &MySQLContestRepository{
		model:    model.NewContestModel(conn),
		cache:    cacheClient,
		ttl:      ttl,
		emptyTTL: emptyTTL,
	}
}

func (r *MySQLContestRepository) Create(ctx context.Context, input ContestCreateInput) error {
	if strings.TrimSpace(input.ContestID) == "" {
		return errors.New("contestID is required")
	}
	if strings.TrimSpace(input.Title) == "" {
		return errors.New("title is required")
	}
	now := time.Now()
	row := model.Contest{
		ContestId:   input.ContestID,
		Title:       input.Title,
		Description: input.Description,
		Status:      input.Status,
		Visibility:  input.Visibility,
		OwnerId:     input.OwnerID,
		OrgId:       input.OrgID,
		StartAt:     input.StartAt,
		EndAt:       input.EndAt,
		RuleJSON:    input.RuleJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.model.Insert(ctx, row); err != nil {
		return err
	}
	if r.cache != nil {
		_ = r.cache.SetWithExpireCtx(ctx, contestDetailKey(input.ContestID), toContestDetail(row), cache_helper.JitterTTL(r.ttl))
	}
	return nil
}

func (r *MySQLContestRepository) Get(ctx context.Context, contestID string) (ContestDetail, error) {
	if strings.TrimSpace(contestID) == "" {
		return ContestDetail{}, errors.New("contestID is required")
	}
	key := contestDetailKey(contestID)
	if r.cache != nil {
		var cached ContestDetail
		if err := r.cache.GetCtx(ctx, key, &cached); err == nil {
			if cached.ContestID == "" {
				return ContestDetail{}, ErrContestNotFound
			}
			return cached, nil
		} else if !r.cache.IsNotFound(err) {
			return ContestDetail{}, err
		}
	}
	row, err := r.model.FindByID(ctx, contestID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			if r.cache != nil {
				_ = r.cache.SetWithExpireCtx(ctx, key, ContestDetail{}, cache_helper.JitterTTL(r.emptyTTL))
			}
			return ContestDetail{}, ErrContestNotFound
		}
		return ContestDetail{}, err
	}
	detail := toContestDetail(row)
	if r.cache != nil {
		_ = r.cache.SetWithExpireCtx(ctx, key, detail, cache_helper.JitterTTL(r.ttl))
	}
	return detail, nil
}

func (r *MySQLContestRepository) List(ctx context.Context, filter ContestListFilter) ([]ContestListItem, int, error) {
	where, args := buildContestListFilter(filter)
	offset := 0
	if filter.Page > 1 {
		offset = (filter.Page - 1) * filter.PageSize
	}
	items, err := r.model.List(ctx, where, args, filter.PageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := r.model.Count(ctx, where, args)
	if err != nil {
		return nil, 0, err
	}

	resp := make([]ContestListItem, 0, len(items))
	for _, item := range items {
		resp = append(resp, ContestListItem{
			ContestID: item.ContestId,
			Title:     item.Title,
			Status:    item.Status,
			StartAt:   item.StartAt,
			EndAt:     item.EndAt,
			RuleJSON:  item.RuleJSON,
		})
	}
	return resp, total, nil
}

func (r *MySQLContestRepository) Update(ctx context.Context, contestID string, update ContestUpdate) error {
	if strings.TrimSpace(contestID) == "" {
		return errors.New("contestID is required")
	}
	setParts := make([]string, 0, 8)
	args := make([]any, 0, 8)
	if update.Title != nil {
		setParts = append(setParts, "title = ?")
		args = append(args, *update.Title)
	}
	if update.Description != nil {
		setParts = append(setParts, "description = ?")
		args = append(args, *update.Description)
	}
	if update.Visibility != nil {
		setParts = append(setParts, "visibility = ?")
		args = append(args, *update.Visibility)
	}
	if update.StartAt != nil {
		setParts = append(setParts, "start_at = ?")
		args = append(args, *update.StartAt)
	}
	if update.EndAt != nil {
		setParts = append(setParts, "end_at = ?")
		args = append(args, *update.EndAt)
	}
	if update.RuleJSON != nil {
		setParts = append(setParts, "rule_json = ?")
		args = append(args, *update.RuleJSON)
	}
	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, contestID)
	setClause := strings.Join(setParts, ", ") + " where contest_id = ?"

	affected, err := r.model.Update(ctx, setClause, args)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrContestNotFound
	}
	return nil
}

func (r *MySQLContestRepository) InvalidateDetailCache(ctx context.Context, contestID string) error {
	if strings.TrimSpace(contestID) == "" {
		return errors.New("contestID is required")
	}
	if r.cache == nil {
		return nil
	}
	return r.cache.DelCtx(ctx, contestDetailKey(contestID))
}

func buildContestListFilter(filter ContestListFilter) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if strings.TrimSpace(filter.Status) != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.OwnerID > 0 {
		clauses = append(clauses, "owner_id = ?")
		args = append(args, filter.OwnerID)
	}
	if filter.OrgID > 0 {
		clauses = append(clauses, "org_id = ?")
		args = append(args, filter.OrgID)
	}
	return strings.Join(clauses, " and "), args
}

func contestDetailKey(contestID string) string {
	return fmt.Sprintf("contest:detail:%s", contestID)
}

func toContestDetail(row model.Contest) ContestDetail {
	return ContestDetail{
		ContestID:   row.ContestId,
		Title:       row.Title,
		Description: row.Description,
		Status:      row.Status,
		Visibility:  row.Visibility,
		OwnerID:     row.OwnerId,
		OrgID:       row.OrgId,
		StartAt:     row.StartAt,
		EndAt:       row.EndAt,
		RuleJSON:    row.RuleJSON,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
