package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProblemVersionModel = (*customProblemVersionModel)(nil)

type (
	// ProblemVersionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProblemVersionModel.
	ProblemVersionModel interface {
		problemVersionModel
		WithSession(session sqlx.Session) ProblemVersionModel
		FindLatestPublishedByProblemID(ctx context.Context, problemID int64) (*ProblemVersion, error)
		FindMetaByProblemIDVersion(ctx context.Context, problemID int64, version int64) (*ProblemVersion, error)
		UpdateDraftMeta(ctx context.Context, problemID int64, version int64, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) (bool, error)
		EnsureDraftVersion(ctx context.Context, problemID int64, version int64, configJSON []byte) error
		PublishVersionIfReady(ctx context.Context, problemID int64, version int64) (bool, error)
	}

	customProblemVersionModel struct {
		*defaultProblemVersionModel
	}
)

// NewProblemVersionModel returns a model for the database table.
func NewProblemVersionModel(conn sqlx.SqlConn) ProblemVersionModel {
	return &customProblemVersionModel{
		defaultProblemVersionModel: newProblemVersionModel(conn),
	}
}

func (m *customProblemVersionModel) WithSession(session sqlx.Session) ProblemVersionModel {
	if session == nil {
		return m
	}
	return NewProblemVersionModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customProblemVersionModel) FindLatestPublishedByProblemID(ctx context.Context, problemID int64) (*ProblemVersion, error) {
	if problemID <= 0 {
		return nil, fmt.Errorf("problemID is required")
	}
	query := `
		SELECT id, problem_id, version, state, config_json, manifest_hash, data_pack_key, data_pack_hash, created_at
		FROM problem_version
		WHERE problem_id = ? AND state = 1
		ORDER BY version DESC
		LIMIT 1`
	var resp ProblemVersion
	err := m.conn.QueryRowCtx(ctx, &resp, query, problemID)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customProblemVersionModel) FindMetaByProblemIDVersion(ctx context.Context, problemID int64, version int64) (*ProblemVersion, error) {
	if problemID <= 0 || version <= 0 {
		return nil, fmt.Errorf("problemID and version are required")
	}
	query := `
		SELECT id, problem_id, version, state, config_json, manifest_hash, data_pack_key, data_pack_hash, created_at
		FROM problem_version
		WHERE problem_id = ? AND version = ?`
	var resp ProblemVersion
	err := m.conn.QueryRowCtx(ctx, &resp, query, problemID, version)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customProblemVersionModel) UpdateDraftMeta(ctx context.Context, problemID int64, version int64, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) (bool, error) {
	if problemID <= 0 || version <= 0 {
		return false, fmt.Errorf("problemID and version are required")
	}
	if len(configJSON) == 0 {
		configJSON = []byte(`{}`)
	}
	res, err := m.conn.ExecCtx(ctx, `
		UPDATE problem_version
		SET config_json = ?, manifest_hash = ?, data_pack_key = ?, data_pack_hash = ?
		WHERE problem_id = ? AND version = ? AND state = 0`,
		configJSON, manifestHash, dataPackKey, dataPackHash, problemID, version,
	)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (m *customProblemVersionModel) EnsureDraftVersion(ctx context.Context, problemID int64, version int64, configJSON []byte) error {
	if problemID <= 0 || version <= 0 {
		return fmt.Errorf("problemID and version are required")
	}
	if len(configJSON) == 0 {
		configJSON = []byte(`{}`)
	}
	_, err := m.conn.ExecCtx(ctx, `
		INSERT INTO problem_version (problem_id, version, state, config_json, manifest_hash, data_pack_key, data_pack_hash)
		VALUES (?, ?, 0, ?, '', '', '')
		ON DUPLICATE KEY UPDATE config_json = config_json`,
		problemID, version, configJSON,
	)
	return err
}

func (m *customProblemVersionModel) PublishVersionIfReady(ctx context.Context, problemID int64, version int64) (bool, error) {
	if problemID <= 0 || version <= 0 {
		return false, fmt.Errorf("problemID and version are required")
	}
	res, err := m.conn.ExecCtx(ctx, `
		UPDATE problem_version
		SET state = 1
		WHERE problem_id = ? AND version = ? AND state = 0
		  AND manifest_hash <> '' AND data_pack_key <> '' AND data_pack_hash <> ''`,
		problemID, version,
	)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
