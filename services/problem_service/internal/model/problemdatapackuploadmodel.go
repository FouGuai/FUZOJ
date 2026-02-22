package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProblemDataPackUploadModel = (*customProblemDataPackUploadModel)(nil)

type (
	// ProblemDataPackUploadModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProblemDataPackUploadModel.
	ProblemDataPackUploadModel interface {
		problemDataPackUploadModel
		WithSession(session sqlx.Session) ProblemDataPackUploadModel
		FindOneByIdempotencyKey(ctx context.Context, problemID int64, idempotencyKey string) (*ProblemDataPackUpload, error)
		InsertUploadSession(ctx context.Context, data *ProblemDataPackUpload) (int64, error)
		FindSessionByID(ctx context.Context, id int64) (*ProblemDataPackUpload, error)
		UpdateUploadIDIfEmpty(ctx context.Context, id int64, uploadID string) (bool, error)
		UpdateState(ctx context.Context, id int64, state int64) (bool, error)
	}

	customProblemDataPackUploadModel struct {
		*defaultProblemDataPackUploadModel
	}
)

// NewProblemDataPackUploadModel returns a model for the database table.
func NewProblemDataPackUploadModel(conn sqlx.SqlConn) ProblemDataPackUploadModel {
	return &customProblemDataPackUploadModel{
		defaultProblemDataPackUploadModel: newProblemDataPackUploadModel(conn),
	}
}

func (m *customProblemDataPackUploadModel) WithSession(session sqlx.Session) ProblemDataPackUploadModel {
	if session == nil {
		return m
	}
	return NewProblemDataPackUploadModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customProblemDataPackUploadModel) FindOneByIdempotencyKey(ctx context.Context, problemID int64, idempotencyKey string) (*ProblemDataPackUpload, error) {
	if problemID <= 0 || idempotencyKey == "" {
		return nil, fmt.Errorf("problemID and idempotencyKey are required")
	}
	return m.FindOneByProblemIdIdempotencyKey(ctx, problemID, idempotencyKey)
}

func (m *customProblemDataPackUploadModel) InsertUploadSession(ctx context.Context, data *ProblemDataPackUpload) (int64, error) {
	if data == nil {
		return 0, fmt.Errorf("upload session is nil")
	}
	if data.ProblemId <= 0 {
		return 0, fmt.Errorf("problemID is required")
	}
	if data.Version <= 0 {
		return 0, fmt.Errorf("version is required")
	}
	if data.IdempotencyKey == "" {
		return 0, fmt.Errorf("idempotencyKey is required")
	}
	if data.Bucket == "" || data.ObjectKey == "" {
		return 0, fmt.Errorf("bucket and objectKey are required")
	}
	if data.ExpiresAt.IsZero() {
		return 0, fmt.Errorf("expiresAt is required")
	}
	res, err := m.Insert(ctx, data)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (m *customProblemDataPackUploadModel) FindSessionByID(ctx context.Context, id int64) (*ProblemDataPackUpload, error) {
	if id <= 0 {
		return nil, fmt.Errorf("uploadSessionID is required")
	}
	return m.FindOne(ctx, id)
}

func (m *customProblemDataPackUploadModel) UpdateUploadIDIfEmpty(ctx context.Context, id int64, uploadID string) (bool, error) {
	if id <= 0 {
		return false, fmt.Errorf("uploadSessionID is required")
	}
	if uploadID == "" {
		return false, fmt.Errorf("uploadID is required")
	}
	res, err := m.conn.ExecCtx(ctx, "UPDATE problem_data_pack_upload SET upload_id = ? WHERE id = ? AND upload_id = ''", uploadID, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (m *customProblemDataPackUploadModel) UpdateState(ctx context.Context, id int64, state int64) (bool, error) {
	if id <= 0 {
		return false, fmt.Errorf("uploadSessionID is required")
	}
	res, err := m.conn.ExecCtx(ctx, "UPDATE problem_data_pack_upload SET state = ? WHERE id = ?", state, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
