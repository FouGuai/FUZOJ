package problem_app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/db"
	"fuzoj/internal/common/storage"
	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"
	"fuzoj/services/problem_service/internal/repository"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"go.uber.org/zap"
)

const (
	defaultUploadKeyPrefix  = "problems"
	defaultPartSizeBytes    = 16 * 1024 * 1024
	defaultUploadSessionTTL = 2 * time.Hour
	defaultPresignTTL       = 15 * time.Minute
)

type problemApp struct {
	conn             sqlx.SqlConn
	repo             repository.ProblemRepository
	uploadRepo       repository.ProblemUploadRepository
	storage          storage.ObjectStorage
	cleanupPublisher cleanupPublisher
	bucket           string
	keyPrefix        string
	partSizeBytes    int64
	sessionTTL       time.Duration
	presignTTL       time.Duration
}

type cleanupPublisher interface {
	PublishProblemDeleted(ctx context.Context, problemID int64) error
}

type CreateInput struct {
	Title   string
	OwnerID int64
}

type PrepareUploadInput struct {
	ProblemID         int64
	IdempotencyKey    string
	ExpectedSizeBytes int64
	ExpectedSHA256    string
	ContentType       string
	CreatedBy         int64
	ClientType        string
	UploadStrategy    string
}

type PrepareUploadOutput struct {
	UploadSessionID   int64
	ProblemID         int64
	Version           int32
	Bucket            string
	ObjectKey         string
	MultipartUploadID string
	PartSizeBytes     int64
	ExpiresAt         time.Time
}

type SignPartsInput struct {
	ProblemID       int64
	UploadSessionID int64
	PartNumbers     []int
}

type SignPartsOutput struct {
	URLs             map[int]string
	ExpiresInSeconds int64
}

type CompletedPartInput struct {
	PartNumber int
	ETag       string
}

type CompleteUploadInput struct {
	ProblemID       int64
	UploadSessionID int64
	Parts           []CompletedPartInput
	ManifestJSON    json.RawMessage
	ConfigJSON      json.RawMessage
	ManifestHash    string
	DataPackHash    string
}

type CompleteUploadOutput struct {
	ProblemID    int64
	Version      int32
	ManifestHash string
	DataPackKey  string
	DataPackHash string
}

type AbortUploadInput struct {
	ProblemID       int64
	UploadSessionID int64
}

type PublishInput struct {
	ProblemID int64
	Version   int32
}

func newProblemApp(repo repository.ProblemRepository, uploadRepo repository.ProblemUploadRepository, storageClient storage.ObjectStorage, publisher cleanupPublisher, conn sqlx.SqlConn, bucket string, keyPrefix string, partSizeBytes int64, sessionTTL, presignTTL time.Duration) *problemApp {
	if keyPrefix == "" {
		keyPrefix = defaultUploadKeyPrefix
	}
	if partSizeBytes <= 0 {
		partSizeBytes = defaultPartSizeBytes
	}
	if sessionTTL <= 0 {
		sessionTTL = defaultUploadSessionTTL
	}
	if presignTTL <= 0 {
		presignTTL = defaultPresignTTL
	}
	return &problemApp{
		conn:             conn,
		repo:             repo,
		uploadRepo:       uploadRepo,
		storage:          storageClient,
		cleanupPublisher: publisher,
		bucket:           bucket,
		keyPrefix:        keyPrefix,
		partSizeBytes:    partSizeBytes,
		sessionTTL:       sessionTTL,
		presignTTL:       presignTTL,
	}
}

func (m *problemApp) GetLatestMeta(ctx context.Context, problemID int64) (repository.ProblemLatestMeta, error) {
	if problemID <= 0 {
		return repository.ProblemLatestMeta{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	meta, err := m.repo.GetLatestMeta(ctx, nil, problemID)
	if err != nil {
		if err == repository.ErrProblemNotFound {
			return repository.ProblemLatestMeta{}, pkgerrors.New(pkgerrors.ProblemNotFound)
		}
		logger.Error(ctx, "get latest meta failed", zap.Int64("problem_id", problemID), zap.Error(err))
		return repository.ProblemLatestMeta{}, pkgerrors.Wrap(fmt.Errorf("get latest meta failed: %w", err), pkgerrors.DatabaseError)
	}
	return meta, nil
}

func (m *problemApp) CreateProblem(ctx context.Context, input CreateInput) (int64, error) {
	if input.Title == "" {
		return 0, pkgerrors.New(pkgerrors.InvalidParams)
	}
	problem := &repository.Problem{
		Title:   input.Title,
		OwnerID: input.OwnerID,
		Status:  repository.ProblemStatusDraft,
	}
	id, err := m.repo.Create(ctx, nil, problem)
	if err != nil {
		logger.Error(ctx, "create problem failed", zap.Error(err))
		return 0, pkgerrors.Wrap(fmt.Errorf("create problem failed: %w", err), pkgerrors.ProblemCreateFailed)
	}
	return id, nil
}

func (m *problemApp) DeleteProblem(ctx context.Context, problemID int64) error {
	if problemID <= 0 {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	if err := m.repo.Delete(ctx, nil, problemID); err != nil {
		if err == repository.ErrProblemNotFound {
			return pkgerrors.New(pkgerrors.ProblemNotFound)
		}
		logger.Error(ctx, "delete problem failed", zap.Int64("problem_id", problemID), zap.Error(err))
		return pkgerrors.Wrap(fmt.Errorf("delete problem failed: %w", err), pkgerrors.ProblemDeleteFailed)
	}
	_ = m.repo.InvalidateLatestMetaCache(ctx, problemID)
	if m.cleanupPublisher != nil {
		if err := m.cleanupPublisher.PublishProblemDeleted(ctx, problemID); err != nil {
			logger.Warn(ctx, "publish cleanup event failed", zap.Int64("problem_id", problemID), zap.Error(err))
		}
	}
	return nil
}

func (m *problemApp) PrepareDataPackUpload(ctx context.Context, input PrepareUploadInput) (PrepareUploadOutput, error) {
	if input.ProblemID <= 0 || input.IdempotencyKey == "" {
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if m.repo != nil {
		exists, err := m.repo.Exists(ctx, nil, input.ProblemID)
		if err != nil {
			logger.Error(ctx, "check problem exists failed", zap.Int64("problem_id", input.ProblemID), zap.Error(err))
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("check problem exists failed: %w", err), pkgerrors.DatabaseError)
		}
		if !exists {
			return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemNotFound)
		}
	}
	if m.storage == nil {
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	if m.bucket == "" {
		return PrepareUploadOutput{}, pkgerrors.Wrap(errors.New("bucket is empty"), pkgerrors.InternalServerError)
	}
	contentType := input.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	now := time.Now()
	expiresAt := now.Add(m.sessionTTL)

	existing, err := m.uploadRepo.GetUploadSessionByIdempotencyKey(ctx, nil, input.ProblemID, input.IdempotencyKey)
	if err == nil {
		if existing.State != repository.UploadStateUploading {
			return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
		}
		if now.After(existing.ExpiresAt) {
			_ = m.uploadRepo.MarkUploadExpired(ctx, nil, existing.ID)
			return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadExpired)
		}
		if existing.UploadID == "" {
			return m.ensureMultipartUpload(ctx, existing, contentType)
		}
		return PrepareUploadOutput{
			UploadSessionID:   existing.ID,
			ProblemID:         existing.ProblemID,
			Version:           existing.Version,
			Bucket:            existing.Bucket,
			ObjectKey:         existing.ObjectKey,
			MultipartUploadID: existing.UploadID,
			PartSizeBytes:     m.partSizeBytes,
			ExpiresAt:         existing.ExpiresAt,
		}, nil
	} else if !errors.Is(err, repository.ErrUploadNotFound) {
		logger.Error(ctx, "get upload session failed", zap.Int64("problem_id", input.ProblemID), zap.Error(err))
		return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}

	var created repository.UploadSession
	if err := m.withTransaction(ctx, func(session sqlx.Session) error {
		ex, err := m.uploadRepo.GetUploadSessionByIdempotencyKey(ctx, session, input.ProblemID, input.IdempotencyKey)
		if err == nil {
			created = ex
			return nil
		} else if !errors.Is(err, repository.ErrUploadNotFound) {
			return err
		}

		version, err := m.uploadRepo.AllocateNextVersion(ctx, session, input.ProblemID)
		if err != nil {
			return err
		}
		objectKey := m.dataPackObjectKey(input.ProblemID, version)

		sessionRecord, err := m.uploadRepo.CreateUploadSession(ctx, session, repository.CreateUploadSessionInput{
			ProblemID:         input.ProblemID,
			Version:           version,
			IdempotencyKey:    input.IdempotencyKey,
			Bucket:            m.bucket,
			ObjectKey:         objectKey,
			ExpiresAt:         expiresAt,
			CreatedBy:         input.CreatedBy,
			ExpectedSizeBytes: input.ExpectedSizeBytes,
			ExpectedSHA256:    input.ExpectedSHA256,
			ContentType:       contentType,
		})
		if err != nil {
			return err
		}
		created = sessionRecord
		return nil
	}); err != nil {
		if key, ok := repositoryUniqueViolation(err); ok && key == "pdu_problem_idem_uq" {
			ex, err2 := m.uploadRepo.GetUploadSessionByIdempotencyKey(ctx, nil, input.ProblemID, input.IdempotencyKey)
			if err2 == nil {
				created = ex
			} else {
				logger.Error(ctx, "retry get upload session failed", zap.Int64("problem_id", input.ProblemID), zap.Error(err2))
				return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("retry get upload session failed: %w", err2), pkgerrors.DatabaseError)
			}
		} else {
			logger.Error(ctx, "create upload session failed", zap.Int64("problem_id", input.ProblemID), zap.Error(err))
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("create upload session failed: %w", err), pkgerrors.DatabaseError)
		}
	}

	return m.ensureMultipartUpload(ctx, created, contentType)
}

func (m *problemApp) SignUploadParts(ctx context.Context, input SignPartsInput) (SignPartsOutput, error) {
	if input.ProblemID <= 0 || input.UploadSessionID <= 0 || len(input.PartNumbers) == 0 {
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if m.storage == nil {
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	session, err := m.uploadRepo.GetUploadSessionByID(ctx, nil, input.UploadSessionID)
	if err != nil {
		if errors.Is(err, repository.ErrUploadNotFound) {
			return SignPartsOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
		}
		logger.Error(ctx, "get upload session failed", zap.Int64("upload_id", input.UploadSessionID), zap.Error(err))
		return SignPartsOutput{}, pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}
	if session.ProblemID != input.ProblemID {
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
	}
	if session.State != repository.UploadStateUploading {
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}
	if time.Now().After(session.ExpiresAt) {
		_ = m.uploadRepo.MarkUploadExpired(ctx, nil, session.ID)
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.ProblemUploadExpired)
	}
	if session.UploadID == "" {
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}

	urls := make(map[int]string, len(input.PartNumbers))
	for _, n := range input.PartNumbers {
		if n <= 0 {
			return SignPartsOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
		}
		u, err := m.storage.PresignUploadPart(ctx, session.Bucket, session.ObjectKey, session.UploadID, n, m.presignTTL, session.ContentType)
		if err != nil {
			logger.Error(ctx, "presign upload part failed", zap.Int("part_number", n), zap.Error(err))
			return SignPartsOutput{}, pkgerrors.Wrap(fmt.Errorf("presign upload part failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
		}
		urls[n] = u
	}
	return SignPartsOutput{
		URLs:             urls,
		ExpiresInSeconds: int64(m.presignTTL.Seconds()),
	}, nil
}

func (m *problemApp) CompleteDataPackUpload(ctx context.Context, input CompleteUploadInput) (CompleteUploadOutput, error) {
	if input.ProblemID <= 0 || input.UploadSessionID <= 0 || len(input.Parts) == 0 {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if m.storage == nil {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	if len(input.ManifestJSON) == 0 || len(input.ConfigJSON) == 0 {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if input.ManifestHash == "" || input.DataPackHash == "" {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}

	session, err := m.uploadRepo.GetUploadSessionByID(ctx, nil, input.UploadSessionID)
	if err != nil {
		if errors.Is(err, repository.ErrUploadNotFound) {
			return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
		}
		logger.Error(ctx, "get upload session failed", zap.Int64("upload_id", input.UploadSessionID), zap.Error(err))
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}
	if session.ProblemID != input.ProblemID {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
	}
	if session.State == repository.UploadStateCompleted {
		meta, err := m.uploadRepo.GetProblemVersionMeta(ctx, nil, session.ProblemID, session.Version)
		if err != nil {
			logger.Error(ctx, "load version meta failed", zap.Int64("problem_id", session.ProblemID), zap.Error(err))
			return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("load version meta failed: %w", err), pkgerrors.DatabaseError)
		}
		return CompleteUploadOutput{
			ProblemID:    session.ProblemID,
			Version:      session.Version,
			ManifestHash: meta.ManifestHash,
			DataPackKey:  meta.DataPackKey,
			DataPackHash: meta.DataPackHash,
		}, nil
	}
	if session.State != repository.UploadStateUploading {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}
	if time.Now().After(session.ExpiresAt) {
		_ = m.uploadRepo.MarkUploadExpired(ctx, nil, session.ID)
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadExpired)
	}
	if session.UploadID == "" {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}
	if session.ExpectedSHA256 != "" && session.ExpectedSHA256 != input.DataPackHash {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadConflict)
	}

	parts := make([]storage.CompletedPart, 0, len(input.Parts))
	for _, p := range input.Parts {
		if p.PartNumber <= 0 || p.ETag == "" {
			return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
		}
		parts = append(parts, storage.CompletedPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		})
	}

	if _, err := m.storage.CompleteMultipartUpload(ctx, session.Bucket, session.ObjectKey, session.UploadID, parts); err != nil {
		logger.Error(ctx, "complete multipart upload failed", zap.Int64("upload_id", session.ID), zap.Error(err))
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("complete multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
	}

	stat, err := m.storage.StatObject(ctx, session.Bucket, session.ObjectKey)
	if err != nil {
		logger.Error(ctx, "stat object failed", zap.String("object_key", session.ObjectKey), zap.Error(err))
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("stat object failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
	}
	if session.ExpectedSizeBytes > 0 && stat.SizeBytes != session.ExpectedSizeBytes {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadConflict)
	}

	if err := m.withTransaction(ctx, func(sessionTx sqlx.Session) error {
		versionID, err := m.uploadRepo.GetProblemVersionID(ctx, sessionTx, session.ProblemID, session.Version)
		if err != nil {
			return err
		}
		if err := m.uploadRepo.UpdateProblemVersionDraftMeta(ctx, sessionTx, session.ProblemID, session.Version, input.ConfigJSON, input.ManifestHash, session.ObjectKey, input.DataPackHash); err != nil {
			return err
		}
		if err := m.uploadRepo.UpsertManifest(ctx, sessionTx, versionID, input.ManifestJSON); err != nil {
			return err
		}
		if err := m.uploadRepo.UpsertDataPack(ctx, sessionTx, versionID, session.ObjectKey, stat.SizeBytes, "", input.DataPackHash); err != nil {
			return err
		}
		if err := m.uploadRepo.MarkUploadCompleted(ctx, sessionTx, session.ID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		logger.Error(ctx, "complete upload persist failed", zap.Int64("upload_id", session.ID), zap.Error(err))
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("complete upload persist failed: %w", err), pkgerrors.DatabaseError)
	}

	return CompleteUploadOutput{
		ProblemID:    session.ProblemID,
		Version:      session.Version,
		ManifestHash: input.ManifestHash,
		DataPackKey:  session.ObjectKey,
		DataPackHash: input.DataPackHash,
	}, nil
}

func (m *problemApp) AbortDataPackUpload(ctx context.Context, input AbortUploadInput) error {
	if input.ProblemID <= 0 || input.UploadSessionID <= 0 {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	if m.storage == nil {
		return pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	session, err := m.uploadRepo.GetUploadSessionByID(ctx, nil, input.UploadSessionID)
	if err != nil {
		if errors.Is(err, repository.ErrUploadNotFound) {
			return pkgerrors.New(pkgerrors.ProblemUploadNotFound)
		}
		logger.Error(ctx, "get upload session failed", zap.Int64("upload_id", input.UploadSessionID), zap.Error(err))
		return pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}
	if session.ProblemID != input.ProblemID {
		return pkgerrors.New(pkgerrors.ProblemUploadNotFound)
	}
	if session.State == repository.UploadStateCompleted {
		return pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}
	if session.UploadID != "" {
		if err := m.storage.AbortMultipartUpload(ctx, session.Bucket, session.ObjectKey, session.UploadID); err != nil {
			logger.Error(ctx, "abort multipart upload failed", zap.Int64("upload_id", session.ID), zap.Error(err))
			return pkgerrors.Wrap(fmt.Errorf("abort multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
		}
	}
	if err := m.uploadRepo.MarkUploadAborted(ctx, nil, session.ID); err != nil {
		logger.Error(ctx, "mark upload aborted failed", zap.Int64("upload_id", session.ID), zap.Error(err))
		return pkgerrors.Wrap(fmt.Errorf("mark upload aborted failed: %w", err), pkgerrors.DatabaseError)
	}
	return nil
}

func (m *problemApp) PublishVersion(ctx context.Context, input PublishInput) error {
	if input.ProblemID <= 0 || input.Version <= 0 {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	if err := m.uploadRepo.PublishVersion(ctx, nil, input.ProblemID, input.Version); err != nil {
		if errors.Is(err, repository.ErrProblemVersionNotFound) {
			return pkgerrors.New(pkgerrors.NotFound)
		}
		if errors.Is(err, repository.ErrProblemVersionNotReady) {
			return pkgerrors.New(pkgerrors.ProblemVersionNotReadyToPublish)
		}
		logger.Error(ctx, "publish version failed", zap.Int64("problem_id", input.ProblemID), zap.Error(err))
		return pkgerrors.Wrap(fmt.Errorf("publish version failed: %w", err), pkgerrors.DatabaseError)
	}
	_ = m.repo.InvalidateLatestMetaCache(ctx, input.ProblemID)
	return nil
}

func (m *problemApp) ensureMultipartUpload(ctx context.Context, session repository.UploadSession, contentType string) (PrepareUploadOutput, error) {
	if session.State != repository.UploadStateUploading {
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}
	if m.storage == nil {
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	if time.Now().After(session.ExpiresAt) {
		_ = m.uploadRepo.MarkUploadExpired(ctx, nil, session.ID)
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadExpired)
	}

	if session.UploadID == "" {
		uploadID, err := m.storage.CreateMultipartUpload(ctx, session.Bucket, session.ObjectKey, contentType)
		if err != nil {
			logger.Error(ctx, "create multipart upload failed", zap.Int64("upload_id", session.ID), zap.Error(err))
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("create multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
		}
		updated, err := m.uploadRepo.UpdateUploadIDIfEmpty(ctx, nil, session.ID, uploadID)
		if err != nil {
			_ = m.storage.AbortMultipartUpload(ctx, session.Bucket, session.ObjectKey, uploadID)
			logger.Error(ctx, "persist upload id failed", zap.Int64("upload_id", session.ID), zap.Error(err))
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("persist upload id failed: %w", err), pkgerrors.DatabaseError)
		}
		if !updated {
			_ = m.storage.AbortMultipartUpload(ctx, session.Bucket, session.ObjectKey, uploadID)
		}
		latest, err := m.uploadRepo.GetUploadSessionByID(ctx, nil, session.ID)
		if err != nil {
			logger.Error(ctx, "reload upload session failed", zap.Int64("upload_id", session.ID), zap.Error(err))
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("reload upload session failed: %w", err), pkgerrors.DatabaseError)
		}
		session = latest
	}

	if session.UploadID == "" {
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}

	return PrepareUploadOutput{
		UploadSessionID:   session.ID,
		ProblemID:         session.ProblemID,
		Version:           session.Version,
		Bucket:            session.Bucket,
		ObjectKey:         session.ObjectKey,
		MultipartUploadID: session.UploadID,
		PartSizeBytes:     m.partSizeBytes,
		ExpiresAt:         session.ExpiresAt,
	}, nil
}

func (m *problemApp) dataPackObjectKey(problemID int64, version int32) string {
	return fmt.Sprintf("%s/%d/versions/%d/data-pack.tar.zst", m.keyPrefix, problemID, version)
}

func (m *problemApp) withTransaction(ctx context.Context, fn func(session sqlx.Session) error) error {
	if m.conn == nil {
		return fn(nil)
	}
	if err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return fn(session)
	}); err != nil {
		return err
	}
	return nil
}

func repositoryUniqueViolation(err error) (string, bool) {
	return db.UniqueViolation(err)
}
