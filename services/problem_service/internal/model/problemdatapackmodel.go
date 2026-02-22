package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProblemDataPackModel = (*customProblemDataPackModel)(nil)

type (
	// ProblemDataPackModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProblemDataPackModel.
	ProblemDataPackModel interface {
		problemDataPackModel
		WithSession(session sqlx.Session) ProblemDataPackModel
		Upsert(ctx context.Context, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error
	}

	customProblemDataPackModel struct {
		*defaultProblemDataPackModel
	}
)

// NewProblemDataPackModel returns a model for the database table.
func NewProblemDataPackModel(conn sqlx.SqlConn) ProblemDataPackModel {
	return &customProblemDataPackModel{
		defaultProblemDataPackModel: newProblemDataPackModel(conn),
	}
}

func (m *customProblemDataPackModel) WithSession(session sqlx.Session) ProblemDataPackModel {
	if session == nil {
		return m
	}
	return NewProblemDataPackModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customProblemDataPackModel) Upsert(ctx context.Context, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error {
	if problemVersionID <= 0 {
		return fmt.Errorf("problemVersionID is required")
	}
	if objectKey == "" {
		return fmt.Errorf("objectKey is required")
	}
	_, err := m.conn.ExecCtx(ctx, `
		INSERT INTO problem_data_pack (problem_version_id, object_key, size_bytes, md5, sha256)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE object_key = VALUES(object_key), size_bytes = VALUES(size_bytes), md5 = VALUES(md5), sha256 = VALUES(sha256)`,
		problemVersionID, objectKey, sizeBytes, md5, sha256,
	)
	return err
}
