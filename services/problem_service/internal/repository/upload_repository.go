package repository

import (
	"context"
	"encoding/json"
	"errors"

	"fuzoj/services/problem_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
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

// ProblemUploadRepository provides upload session and version write operations.
type ProblemUploadRepository interface {
	AllocateNextVersion(ctx context.Context, session sqlx.Session, problemID int64) (int32, error)
	GetUploadSessionByIdempotencyKey(ctx context.Context, session sqlx.Session, problemID int64, idempotencyKey string) (UploadSession, error)
	CreateUploadSession(ctx context.Context, session sqlx.Session, input CreateUploadSessionInput) (UploadSession, error)
	GetUploadSessionByID(ctx context.Context, session sqlx.Session, uploadSessionID int64) (UploadSession, error)
	UpdateUploadIDIfEmpty(ctx context.Context, session sqlx.Session, uploadSessionID int64, uploadID string) (bool, error)
	MarkUploadCompleted(ctx context.Context, session sqlx.Session, uploadSessionID int64) error
	MarkUploadAborted(ctx context.Context, session sqlx.Session, uploadSessionID int64) error
	MarkUploadExpired(ctx context.Context, session sqlx.Session, uploadSessionID int64) error

	GetProblemVersionMeta(ctx context.Context, session sqlx.Session, problemID int64, version int32) (ProblemVersionMeta, error)
	GetProblemVersionID(ctx context.Context, session sqlx.Session, problemID int64, version int32) (int64, error)
	UpdateProblemVersionDraftMeta(ctx context.Context, session sqlx.Session, problemID int64, version int32, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) error
	UpsertManifest(ctx context.Context, session sqlx.Session, problemVersionID int64, manifestJSON []byte) error
	UpsertDataPack(ctx context.Context, session sqlx.Session, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error
	PublishVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) error
}

type MySQLProblemUploadRepository struct {
	uploadModel     model.ProblemDataPackUploadModel
	versionModel    model.ProblemVersionModel
	versionSeqModel model.ProblemVersionSeqModel
	manifestModel   model.ProblemManifestModel
	dataPackModel   model.ProblemDataPackModel
}

func NewProblemUploadRepository(conn sqlx.SqlConn) ProblemUploadRepository {
	return &MySQLProblemUploadRepository{
		uploadModel:     model.NewProblemDataPackUploadModel(conn),
		versionModel:    model.NewProblemVersionModel(conn),
		versionSeqModel: model.NewProblemVersionSeqModel(conn),
		manifestModel:   model.NewProblemManifestModel(conn),
		dataPackModel:   model.NewProblemDataPackModel(conn),
	}
}

func (r *MySQLProblemUploadRepository) AllocateNextVersion(ctx context.Context, session sqlx.Session, problemID int64) (int32, error) {
	version, err := r.versionSeqModel.WithSession(session).AllocateNextVersion(ctx, problemID)
	if err != nil {
		return 0, err
	}
	return int32(version), nil
}

func (r *MySQLProblemUploadRepository) GetUploadSessionByIdempotencyKey(ctx context.Context, session sqlx.Session, problemID int64, idempotencyKey string) (UploadSession, error) {
	return r.getUploadSessionByIdempotencyKey(ctx, session, problemID, idempotencyKey)
}

func (r *MySQLProblemUploadRepository) CreateUploadSession(ctx context.Context, session sqlx.Session, input CreateUploadSessionInput) (UploadSession, error) {
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

	uploadSessionID, err := r.uploadModel.WithSession(session).InsertUploadSession(ctx, &model.ProblemDataPackUpload{
		ProblemId:         input.ProblemID,
		Version:           int64(input.Version),
		IdempotencyKey:    input.IdempotencyKey,
		Bucket:            input.Bucket,
		ObjectKey:         input.ObjectKey,
		UploadId:          "",
		ExpectedSizeBytes: input.ExpectedSizeBytes,
		ExpectedSha256:    input.ExpectedSHA256,
		ContentType:       input.ContentType,
		State:             int64(UploadStateUploading),
		ExpiresAt:         input.ExpiresAt,
		CreatedBy:         input.CreatedBy,
	})
	if err != nil {
		return UploadSession{}, err
	}

	emptyConfig, _ := json.Marshal(map[string]interface{}{})
	if err := r.versionModel.WithSession(session).EnsureDraftVersion(ctx, input.ProblemID, int64(input.Version), emptyConfig); err != nil {
		return UploadSession{}, err
	}

	return r.GetUploadSessionByID(ctx, session, uploadSessionID)
}

func (r *MySQLProblemUploadRepository) GetUploadSessionByID(ctx context.Context, session sqlx.Session, uploadSessionID int64) (UploadSession, error) {
	row, err := r.uploadModel.WithSession(session).FindSessionByID(ctx, uploadSessionID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return UploadSession{}, ErrUploadNotFound
		}
		return UploadSession{}, err
	}
	return toUploadSession(row), nil
}

func (r *MySQLProblemUploadRepository) UpdateUploadIDIfEmpty(ctx context.Context, session sqlx.Session, uploadSessionID int64, uploadID string) (bool, error) {
	return r.uploadModel.WithSession(session).UpdateUploadIDIfEmpty(ctx, uploadSessionID, uploadID)
}

func (r *MySQLProblemUploadRepository) MarkUploadCompleted(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
	return r.updateUploadState(ctx, session, uploadSessionID, UploadStateCompleted)
}

func (r *MySQLProblemUploadRepository) MarkUploadAborted(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
	return r.updateUploadState(ctx, session, uploadSessionID, UploadStateAborted)
}

func (r *MySQLProblemUploadRepository) MarkUploadExpired(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
	return r.updateUploadState(ctx, session, uploadSessionID, UploadStateExpired)
}

func (r *MySQLProblemUploadRepository) updateUploadState(ctx context.Context, session sqlx.Session, uploadSessionID int64, state int32) error {
	updated, err := r.uploadModel.WithSession(session).UpdateState(ctx, uploadSessionID, int64(state))
	if err != nil {
		return err
	}
	if !updated {
		return ErrUploadNotFound
	}
	return nil
}

func (r *MySQLProblemUploadRepository) GetProblemVersionID(ctx context.Context, session sqlx.Session, problemID int64, version int32) (int64, error) {
	row, err := r.versionModel.WithSession(session).FindMetaByProblemIDVersion(ctx, problemID, int64(version))
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return 0, ErrProblemVersionNotFound
		}
		return 0, err
	}
	return row.Id, nil
}

func (r *MySQLProblemUploadRepository) GetProblemVersionMeta(ctx context.Context, session sqlx.Session, problemID int64, version int32) (ProblemVersionMeta, error) {
	row, err := r.versionModel.WithSession(session).FindMetaByProblemIDVersion(ctx, problemID, int64(version))
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ProblemVersionMeta{}, ErrProblemVersionNotFound
		}
		return ProblemVersionMeta{}, err
	}
	return ProblemVersionMeta{
		ProblemID:    row.ProblemId,
		Version:      int32(row.Version),
		State:        int32(row.State),
		ManifestHash: row.ManifestHash,
		DataPackKey:  row.DataPackKey,
		DataPackHash: row.DataPackHash,
	}, nil
}

func (r *MySQLProblemUploadRepository) UpdateProblemVersionDraftMeta(ctx context.Context, session sqlx.Session, problemID int64, version int32, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) error {
	updated, err := r.versionModel.WithSession(session).UpdateDraftMeta(ctx, problemID, int64(version), configJSON, manifestHash, dataPackKey, dataPackHash)
	if err != nil {
		return err
	}
	if !updated {
		return ErrProblemVersionNotFound
	}
	return nil
}

func (r *MySQLProblemUploadRepository) UpsertManifest(ctx context.Context, session sqlx.Session, problemVersionID int64, manifestJSON []byte) error {
	return r.manifestModel.WithSession(session).Upsert(ctx, problemVersionID, manifestJSON)
}

func (r *MySQLProblemUploadRepository) UpsertDataPack(ctx context.Context, session sqlx.Session, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error {
	return r.dataPackModel.WithSession(session).Upsert(ctx, problemVersionID, objectKey, sizeBytes, md5, sha256)
}

func (r *MySQLProblemUploadRepository) PublishVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) error {
	updated, err := r.versionModel.WithSession(session).PublishVersionIfReady(ctx, problemID, int64(version))
	if err != nil {
		return err
	}
	if !updated {
		if _, err := r.versionModel.WithSession(session).FindMetaByProblemIDVersion(ctx, problemID, int64(version)); err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return ErrProblemVersionNotFound
			}
			return err
		}
		return ErrProblemVersionNotReady
	}
	return nil
}

func (r *MySQLProblemUploadRepository) getUploadSessionByIdempotencyKey(ctx context.Context, session sqlx.Session, problemID int64, idempotencyKey string) (UploadSession, error) {
	row, err := r.uploadModel.WithSession(session).FindOneByIdempotencyKey(ctx, problemID, idempotencyKey)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return UploadSession{}, ErrUploadNotFound
		}
		return UploadSession{}, err
	}
	return toUploadSession(row), nil
}

func toUploadSession(row *model.ProblemDataPackUpload) UploadSession {
	if row == nil {
		return UploadSession{}
	}
	return UploadSession{
		ID:                row.Id,
		ProblemID:         row.ProblemId,
		Version:           int32(row.Version),
		IdempotencyKey:    row.IdempotencyKey,
		Bucket:            row.Bucket,
		ObjectKey:         row.ObjectKey,
		UploadID:          row.UploadId,
		ExpectedSizeBytes: row.ExpectedSizeBytes,
		ExpectedSHA256:    row.ExpectedSha256,
		ContentType:       row.ContentType,
		State:             int32(row.State),
		ExpiresAt:         row.ExpiresAt,
		CreatedBy:         row.CreatedBy,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

// time parsing is handled by sqlx into time.Time fields.
