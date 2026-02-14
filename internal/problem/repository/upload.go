package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/db"
)

const (
	UploadStateUploading int32 = 0
	UploadStateCompleted int32 = 1
	UploadStateAborted   int32 = 2
	UploadStateExpired   int32 = 3
)

var (
	ErrUploadNotFound         = errors.New("upload session not found")
	ErrUploadExpired          = errors.New("upload session expired")
	ErrUploadStateInvalid     = errors.New("upload session state invalid")
	ErrUploadConflict         = errors.New("upload conflict")
	ErrProblemVersionNotFound = errors.New("problem version not found")
	ErrProblemVersionNotReady = errors.New("problem version not ready")
)

// UploadSession represents one multipart upload session.
type UploadSession struct {
	ID                int64
	ProblemID         int64
	Version           int32
	IdempotencyKey    string
	Bucket            string
	ObjectKey         string
	UploadID          string
	ExpectedSizeBytes int64
	ExpectedSHA256    string
	ContentType       string
	State             int32
	ExpiresAt         time.Time
	CreatedBy         int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ProblemUploadRepository provides upload session and version write operations.
type ProblemUploadRepository interface {
	AllocateNextVersion(ctx context.Context, tx db.Transaction, problemID int64) (int32, error)
	GetUploadSessionByIdempotencyKey(ctx context.Context, tx db.Transaction, problemID int64, idempotencyKey string) (UploadSession, error)
	CreateUploadSession(ctx context.Context, tx db.Transaction, input CreateUploadSessionInput) (UploadSession, error)
	GetUploadSessionByID(ctx context.Context, tx db.Transaction, uploadSessionID int64) (UploadSession, error)
	UpdateUploadIDIfEmpty(ctx context.Context, tx db.Transaction, uploadSessionID int64, uploadID string) (bool, error)
	MarkUploadCompleted(ctx context.Context, tx db.Transaction, uploadSessionID int64) error
	MarkUploadAborted(ctx context.Context, tx db.Transaction, uploadSessionID int64) error
	MarkUploadExpired(ctx context.Context, tx db.Transaction, uploadSessionID int64) error

	GetProblemVersionMeta(ctx context.Context, tx db.Transaction, problemID int64, version int32) (ProblemVersionMeta, error)
	GetProblemVersionID(ctx context.Context, tx db.Transaction, problemID int64, version int32) (int64, error)
	UpdateProblemVersionDraftMeta(ctx context.Context, tx db.Transaction, problemID int64, version int32, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) error
	UpsertManifest(ctx context.Context, tx db.Transaction, problemVersionID int64, manifestJSON []byte) error
	UpsertDataPack(ctx context.Context, tx db.Transaction, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error
	PublishVersion(ctx context.Context, tx db.Transaction, problemID int64, version int32) error
}

type ProblemVersionMeta struct {
	ProblemID    int64
	Version      int32
	State        int32
	ManifestHash string
	DataPackKey  string
	DataPackHash string
}

type CreateUploadSessionInput struct {
	ProblemID      int64
	Version        int32
	IdempotencyKey string
	Bucket         string
	ObjectKey      string
	ExpiresAt      time.Time
	CreatedBy      int64

	ExpectedSizeBytes int64
	ExpectedSHA256    string
	ContentType       string
}

type MySQLProblemUploadRepository struct {
	db db.Database
}

func NewProblemUploadRepository(database db.Database) ProblemUploadRepository {
	return &MySQLProblemUploadRepository{db: database}
}

func (r *MySQLProblemUploadRepository) AllocateNextVersion(ctx context.Context, tx db.Transaction, problemID int64) (int32, error) {
	if problemID <= 0 {
		return 0, errors.New("problemID is required")
	}

	q := db.GetQuerier(r.db, tx)

	// Ensure row exists.
	_, err := q.Exec(ctx, "INSERT IGNORE INTO problem_version_seq (problem_id, next_version) VALUES (?, 1)", problemID)
	if err != nil {
		return 0, err
	}
	// Atomically increment next_version and expose it via LAST_INSERT_ID.
	_, err = q.Exec(ctx, "UPDATE problem_version_seq SET next_version = LAST_INSERT_ID(next_version + 1) WHERE problem_id = ?", problemID)
	if err != nil {
		return 0, err
	}
	var next int64
	if err := q.QueryRow(ctx, "SELECT LAST_INSERT_ID()").Scan(&next); err != nil {
		return 0, err
	}
	version := next - 1
	if version <= 0 {
		return 0, fmt.Errorf("invalid allocated version: %d", version)
	}
	return int32(version), nil
}

func (r *MySQLProblemUploadRepository) GetUploadSessionByIdempotencyKey(ctx context.Context, tx db.Transaction, problemID int64, idempotencyKey string) (UploadSession, error) {
	return r.getUploadSessionByIdempotencyKey(ctx, tx, problemID, idempotencyKey)
}

func (r *MySQLProblemUploadRepository) CreateUploadSession(ctx context.Context, tx db.Transaction, input CreateUploadSessionInput) (UploadSession, error) {
	if input.ProblemID <= 0 {
		return UploadSession{}, errors.New("problemID is required")
	}
	if input.Version <= 0 {
		return UploadSession{}, errors.New("version is required")
	}
	if input.IdempotencyKey == "" {
		return UploadSession{}, errors.New("idempotencyKey is required")
	}
	if input.Bucket == "" || input.ObjectKey == "" {
		return UploadSession{}, errors.New("bucket and objectKey are required")
	}
	if input.ExpiresAt.IsZero() {
		return UploadSession{}, errors.New("expiresAt is required")
	}

	q := db.GetQuerier(r.db, tx)

	res, err := q.Exec(ctx, `
		INSERT INTO problem_data_pack_upload
			(problem_id, version, idempotency_key, bucket, object_key, upload_id,
			 expected_size_bytes, expected_sha256, content_type,
			 state, expires_at, created_by)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, ?, ?, ?)`,
		input.ProblemID, input.Version, input.IdempotencyKey, input.Bucket, input.ObjectKey,
		input.ExpectedSizeBytes, input.ExpectedSHA256, input.ContentType,
		UploadStateUploading, input.ExpiresAt, input.CreatedBy,
	)
	if err != nil {
		return UploadSession{}, err
	}
	uploadSessionID, err := res.LastInsertId()
	if err != nil {
		return UploadSession{}, err
	}

	// Ensure a draft version row exists. Use placeholder values for NOT NULL columns.
	emptyConfig, _ := json.Marshal(map[string]interface{}{})
	_, err = q.Exec(ctx, `
		INSERT INTO problem_version (problem_id, version, state, config_json, manifest_hash, data_pack_key, data_pack_hash)
		VALUES (?, ?, 0, ?, '', '', '')
		ON DUPLICATE KEY UPDATE config_json = config_json`,
		input.ProblemID, input.Version, emptyConfig,
	)
	if err != nil {
		return UploadSession{}, err
	}

	return r.GetUploadSessionByID(ctx, tx, uploadSessionID)
}

func (r *MySQLProblemUploadRepository) GetUploadSessionByID(ctx context.Context, tx db.Transaction, uploadSessionID int64) (UploadSession, error) {
	q := db.GetQuerier(r.db, tx)
	row := q.QueryRow(ctx, `
		SELECT id, problem_id, version, idempotency_key, bucket, object_key, upload_id,
		       expected_size_bytes, expected_sha256, content_type,
		       state, expires_at, created_by, created_at, updated_at
		FROM problem_data_pack_upload
		WHERE id = ?`, uploadSessionID)

	var s UploadSession
	if err := row.Scan(
		&s.ID,
		&s.ProblemID,
		&s.Version,
		&s.IdempotencyKey,
		&s.Bucket,
		&s.ObjectKey,
		&s.UploadID,
		&s.ExpectedSizeBytes,
		&s.ExpectedSHA256,
		&s.ContentType,
		&s.State,
		&s.ExpiresAt,
		&s.CreatedBy,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return UploadSession{}, ErrUploadNotFound
		}
		return UploadSession{}, err
	}
	return s, nil
}

func (r *MySQLProblemUploadRepository) UpdateUploadIDIfEmpty(ctx context.Context, tx db.Transaction, uploadSessionID int64, uploadID string) (bool, error) {
	if uploadID == "" {
		return false, errors.New("uploadID is required")
	}
	q := db.GetQuerier(r.db, tx)
	res, err := q.Exec(ctx, "UPDATE problem_data_pack_upload SET upload_id = ? WHERE id = ? AND upload_id = ''", uploadID, uploadSessionID)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *MySQLProblemUploadRepository) MarkUploadCompleted(ctx context.Context, tx db.Transaction, uploadSessionID int64) error {
	return r.updateUploadState(ctx, tx, uploadSessionID, UploadStateCompleted)
}

func (r *MySQLProblemUploadRepository) MarkUploadAborted(ctx context.Context, tx db.Transaction, uploadSessionID int64) error {
	return r.updateUploadState(ctx, tx, uploadSessionID, UploadStateAborted)
}

func (r *MySQLProblemUploadRepository) MarkUploadExpired(ctx context.Context, tx db.Transaction, uploadSessionID int64) error {
	return r.updateUploadState(ctx, tx, uploadSessionID, UploadStateExpired)
}

func (r *MySQLProblemUploadRepository) updateUploadState(ctx context.Context, tx db.Transaction, uploadSessionID int64, state int32) error {
	q := db.GetQuerier(r.db, tx)
	res, err := q.Exec(ctx, "UPDATE problem_data_pack_upload SET state = ? WHERE id = ?", state, uploadSessionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUploadNotFound
	}
	return nil
}

func (r *MySQLProblemUploadRepository) GetProblemVersionID(ctx context.Context, tx db.Transaction, problemID int64, version int32) (int64, error) {
	q := db.GetQuerier(r.db, tx)
	row := q.QueryRow(ctx, "SELECT id FROM problem_version WHERE problem_id = ? AND version = ?", problemID, version)
	var id int64
	if err := row.Scan(&id); err != nil {
		if db.IsNoRows(err) {
			return 0, ErrProblemVersionNotFound
		}
		return 0, err
	}
	return id, nil
}

func (r *MySQLProblemUploadRepository) GetProblemVersionMeta(ctx context.Context, tx db.Transaction, problemID int64, version int32) (ProblemVersionMeta, error) {
	q := db.GetQuerier(r.db, tx)
	row := q.QueryRow(ctx, `
		SELECT problem_id, version, state, manifest_hash, data_pack_key, data_pack_hash
		FROM problem_version
		WHERE problem_id = ? AND version = ?`, problemID, version)
	var m ProblemVersionMeta
	if err := row.Scan(&m.ProblemID, &m.Version, &m.State, &m.ManifestHash, &m.DataPackKey, &m.DataPackHash); err != nil {
		if db.IsNoRows(err) {
			return ProblemVersionMeta{}, ErrProblemVersionNotFound
		}
		return ProblemVersionMeta{}, err
	}
	return m, nil
}

func (r *MySQLProblemUploadRepository) UpdateProblemVersionDraftMeta(ctx context.Context, tx db.Transaction, problemID int64, version int32, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) error {
	if len(configJSON) == 0 {
		configJSON = []byte(`{}`)
	}
	q := db.GetQuerier(r.db, tx)
	res, err := q.Exec(ctx, `
		UPDATE problem_version
		SET config_json = ?, manifest_hash = ?, data_pack_key = ?, data_pack_hash = ?
		WHERE problem_id = ? AND version = ? AND state = 0`,
		configJSON, manifestHash, dataPackKey, dataPackHash, problemID, version,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrProblemVersionNotFound
	}
	return nil
}

func (r *MySQLProblemUploadRepository) UpsertManifest(ctx context.Context, tx db.Transaction, problemVersionID int64, manifestJSON []byte) error {
	q := db.GetQuerier(r.db, tx)
	_, err := q.Exec(ctx, `
		INSERT INTO problem_manifest (problem_version_id, manifest_json)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE manifest_json = VALUES(manifest_json)`,
		problemVersionID, manifestJSON,
	)
	return err
}

func (r *MySQLProblemUploadRepository) UpsertDataPack(ctx context.Context, tx db.Transaction, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error {
	q := db.GetQuerier(r.db, tx)
	_, err := q.Exec(ctx, `
		INSERT INTO problem_data_pack (problem_version_id, object_key, size_bytes, md5, sha256)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE object_key = VALUES(object_key), size_bytes = VALUES(size_bytes), md5 = VALUES(md5), sha256 = VALUES(sha256)`,
		problemVersionID, objectKey, sizeBytes, md5, sha256,
	)
	return err
}

func (r *MySQLProblemUploadRepository) PublishVersion(ctx context.Context, tx db.Transaction, problemID int64, version int32) error {
	q := db.GetQuerier(r.db, tx)
	res, err := q.Exec(ctx, `
		UPDATE problem_version
		SET state = 1
		WHERE problem_id = ? AND version = ? AND state = 0
		  AND manifest_hash <> '' AND data_pack_key <> '' AND data_pack_hash <> ''`,
		problemID, version,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Distinguish "not found" from "not ready".
		var exists int
		if err := q.QueryRow(ctx, "SELECT 1 FROM problem_version WHERE problem_id = ? AND version = ?", problemID, version).Scan(&exists); err != nil {
			if db.IsNoRows(err) {
				return ErrProblemVersionNotFound
			}
			return err
		}
		return ErrProblemVersionNotReady
	}
	return nil
}

func (r *MySQLProblemUploadRepository) getUploadSessionByIdempotencyKey(ctx context.Context, tx db.Transaction, problemID int64, idempotencyKey string) (UploadSession, error) {
	q := db.GetQuerier(r.db, tx)
	row := q.QueryRow(ctx, `
		SELECT id, problem_id, version, idempotency_key, bucket, object_key, upload_id,
		       expected_size_bytes, expected_sha256, content_type,
		       state, expires_at, created_by, created_at, updated_at
		FROM problem_data_pack_upload
		WHERE problem_id = ? AND idempotency_key = ?`, problemID, idempotencyKey)

	var s UploadSession
	if err := row.Scan(
		&s.ID,
		&s.ProblemID,
		&s.Version,
		&s.IdempotencyKey,
		&s.Bucket,
		&s.ObjectKey,
		&s.UploadID,
		&s.ExpectedSizeBytes,
		&s.ExpectedSHA256,
		&s.ContentType,
		&s.State,
		&s.ExpiresAt,
		&s.CreatedBy,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return UploadSession{}, ErrUploadNotFound
		}
		return UploadSession{}, err
	}
	return s, nil
}
