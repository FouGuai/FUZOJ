package repository

import (
	"context"
	"errors"

	"fuzoj/services/contest_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrContestProblemNotFound = errors.New("contest problem not found")

type ContestProblemItem struct {
	ProblemID int64
	Order     int
	Score     int
	Visible   bool
	Version   int32
}

type ContestProblemStore interface {
	Upsert(ctx context.Context, contestID string, item ContestProblemItem) error
	Update(ctx context.Context, contestID string, item ContestProblemItem) error
	Remove(ctx context.Context, contestID string, problemID int64) error
	List(ctx context.Context, contestID string) ([]ContestProblemItem, error)
}

type MySQLContestProblemStore struct {
	model *model.ContestProblemModel
}

func NewContestProblemStore(conn sqlx.SqlConn) ContestProblemStore {
	return &MySQLContestProblemStore{
		model: model.NewContestProblemModel(conn),
	}
}

func (r *MySQLContestProblemStore) Upsert(ctx context.Context, contestID string, item ContestProblemItem) error {
	return r.model.Upsert(ctx, model.ContestProblem{
		ContestId: contestID,
		ProblemId: item.ProblemID,
		Order:     item.Order,
		Score:     item.Score,
		Visible:   item.Visible,
		Version:   item.Version,
	})
}

func (r *MySQLContestProblemStore) Update(ctx context.Context, contestID string, item ContestProblemItem) error {
	affected, err := r.model.Update(ctx, model.ContestProblem{
		ContestId: contestID,
		ProblemId: item.ProblemID,
		Order:     item.Order,
		Score:     item.Score,
		Visible:   item.Visible,
		Version:   item.Version,
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrContestProblemNotFound
	}
	return nil
}

func (r *MySQLContestProblemStore) Remove(ctx context.Context, contestID string, problemID int64) error {
	_, err := r.model.Delete(ctx, contestID, problemID)
	return err
}

func (r *MySQLContestProblemStore) List(ctx context.Context, contestID string) ([]ContestProblemItem, error) {
	rows, err := r.model.ListByContest(ctx, contestID)
	if err != nil {
		return nil, err
	}
	items := make([]ContestProblemItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ContestProblemItem{
			ProblemID: row.ProblemId,
			Order:     row.Order,
			Score:     row.Score,
			Visible:   row.Visible,
			Version:   row.Version,
		})
	}
	return items, nil
}
