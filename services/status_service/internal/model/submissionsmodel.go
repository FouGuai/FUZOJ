package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SubmissionsModel = (*customSubmissionsModel)(nil)

type (
	// SubmissionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSubmissionsModel.
	SubmissionsModel interface {
		submissionsModel
	}

	customSubmissionsModel struct {
		*defaultSubmissionsModel
	}
)

// SubmissionFinalStatus represents a submission with final status payload.
type SubmissionFinalStatus struct {
	SubmissionID string `db:"submission_id"`
	FinalStatus  string `db:"final_status"`
}

// NewSubmissionsModel returns a model for the database table.
func NewSubmissionsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SubmissionsModel {
	return &customSubmissionsModel{
		defaultSubmissionsModel: newSubmissionsModel(conn, c, opts...),
	}
}

func (m *customSubmissionsModel) FindFinalStatus(ctx context.Context, submissionID string) (string, error) {
	return m.defaultSubmissionsModel.FindFinalStatus(ctx, submissionID)
}

func (m *customSubmissionsModel) FindFinalStatusBatch(ctx context.Context, submissionIDs []string) ([]SubmissionFinalStatus, error) {
	return m.defaultSubmissionsModel.FindFinalStatusBatch(ctx, submissionIDs)
}

func (m *customSubmissionsModel) UpdateFinalStatus(ctx context.Context, submissionID string, payload string, finishedAt time.Time) (sql.Result, error) {
	return m.defaultSubmissionsModel.UpdateFinalStatus(ctx, submissionID, payload, finishedAt)
}
