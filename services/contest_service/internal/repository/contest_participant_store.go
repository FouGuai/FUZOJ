package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"fuzoj/services/contest_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrContestParticipantNotFound = errors.New("contest participant not found")

type ContestParticipantItem struct {
	UserID       int64
	TeamID       string
	Status       string
	RegisteredAt time.Time
}

type ContestParticipantStore interface {
	Upsert(ctx context.Context, contestID string, item ContestParticipantItem) error
	Find(ctx context.Context, contestID string, userID int64) (ContestParticipantItem, error)
	List(ctx context.Context, contestID string, page, pageSize int) ([]ContestParticipantItem, int, error)
}

type MySQLContestParticipantStore struct {
	model *model.ContestParticipantModel
}

func NewContestParticipantStore(conn sqlx.SqlConn) ContestParticipantStore {
	return &MySQLContestParticipantStore{
		model: model.NewContestParticipantModel(conn),
	}
}

func (r *MySQLContestParticipantStore) Upsert(ctx context.Context, contestID string, item ContestParticipantItem) error {
	teamID := sql.NullString{}
	if item.TeamID != "" {
		teamID = sql.NullString{String: item.TeamID, Valid: true}
	}
	return r.model.Upsert(ctx, model.ContestParticipant{
		ContestId:    contestID,
		UserId:       item.UserID,
		TeamId:       teamID,
		Status:       item.Status,
		RegisteredAt: item.RegisteredAt,
	})
}

func (r *MySQLContestParticipantStore) Find(ctx context.Context, contestID string, userID int64) (ContestParticipantItem, error) {
	row, err := r.model.FindOne(ctx, contestID, userID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return ContestParticipantItem{}, ErrContestParticipantNotFound
		}
		return ContestParticipantItem{}, err
	}
	item := ContestParticipantItem{
		UserID:       row.UserId,
		Status:       row.Status,
		RegisteredAt: row.RegisteredAt,
	}
	if row.TeamId.Valid {
		item.TeamID = row.TeamId.String
	}
	return item, nil
}

func (r *MySQLContestParticipantStore) List(ctx context.Context, contestID string, page, pageSize int) ([]ContestParticipantItem, int, error) {
	offset := 0
	if page > 1 {
		offset = (page - 1) * pageSize
	}
	rows, err := r.model.ListByContest(ctx, contestID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := r.model.CountByContest(ctx, contestID)
	if err != nil {
		return nil, 0, err
	}
	items := make([]ContestParticipantItem, 0, len(rows))
	for _, row := range rows {
		item := ContestParticipantItem{
			UserID:       row.UserId,
			Status:       row.Status,
			RegisteredAt: row.RegisteredAt,
		}
		if row.TeamId.Valid {
			item.TeamID = row.TeamId.String
		}
		items = append(items, item)
	}
	return items, total, nil
}
