package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProblemManifestModel = (*customProblemManifestModel)(nil)

type (
	// ProblemManifestModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProblemManifestModel.
	ProblemManifestModel interface {
		problemManifestModel
		WithSession(session sqlx.Session) ProblemManifestModel
		Upsert(ctx context.Context, problemVersionID int64, manifestJSON []byte) error
	}

	customProblemManifestModel struct {
		*defaultProblemManifestModel
	}
)

// NewProblemManifestModel returns a model for the database table.
func NewProblemManifestModel(conn sqlx.SqlConn) ProblemManifestModel {
	return &customProblemManifestModel{
		defaultProblemManifestModel: newProblemManifestModel(conn),
	}
}

func (m *customProblemManifestModel) WithSession(session sqlx.Session) ProblemManifestModel {
	if session == nil {
		return m
	}
	return NewProblemManifestModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customProblemManifestModel) Upsert(ctx context.Context, problemVersionID int64, manifestJSON []byte) error {
	if problemVersionID <= 0 {
		return fmt.Errorf("problemVersionID is required")
	}
	_, err := m.conn.ExecCtx(ctx, `
		INSERT INTO problem_manifest (problem_version_id, manifest_json)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE manifest_json = VALUES(manifest_json)`,
		problemVersionID, manifestJSON,
	)
	return err
}
