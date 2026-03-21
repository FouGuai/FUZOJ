package problem_app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/db"
	"fuzoj/internal/common/storage"
	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/problem_service/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	defaultUploadKeyPrefix  = "problems"
	defaultPartSizeBytes    = 16 * 1024 * 1024
	defaultUploadSessionTTL = 2 * time.Hour
	defaultPresignTTL       = 15 * time.Minute
)

type problemApp struct {
	conn              sqlx.SqlConn
	repo              repository.ProblemRepository
	statementRepo     repository.ProblemStatementRepository
	uploadRepo        repository.ProblemUploadRepository
	storage           storage.ObjectStorage
	cleanupPublisher  cleanupPublisher
	metaPublisher     metaInvalidationPublisher
	bucket            string
	keyPrefix         string
	partSizeBytes     int64
	sessionTTL        time.Duration
	presignTTL        time.Duration
	statementMaxBytes int
}

type cleanupPublisher interface {
	PublishProblemDeleted(ctx context.Context, problemID int64) error
}

type metaInvalidationPublisher interface {
	PublishProblemMetaInvalidated(ctx context.Context, problemID int64, version int32) error
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

func newProblemApp(repo repository.ProblemRepository, statementRepo repository.ProblemStatementRepository, uploadRepo repository.ProblemUploadRepository, storageClient storage.ObjectStorage, publisher cleanupPublisher, metaPublisher metaInvalidationPublisher, conn sqlx.SqlConn, bucket string, keyPrefix string, partSizeBytes int64, sessionTTL, presignTTL time.Duration, statementMaxBytes int) *problemApp {
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
		conn:              conn,
		repo:              repo,
		statementRepo:     statementRepo,
		uploadRepo:        uploadRepo,
		storage:           storageClient,
		cleanupPublisher:  publisher,
		metaPublisher:     metaPublisher,
		bucket:            bucket,
		keyPrefix:         keyPrefix,
		partSizeBytes:     partSizeBytes,
		sessionTTL:        sessionTTL,
		presignTTL:        presignTTL,
		statementMaxBytes: statementMaxBytes,
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
		logx.WithContext(ctx).Errorf("get latest meta failed problem_id=%d err=%v", problemID, err)
		return repository.ProblemLatestMeta{}, pkgerrors.Wrap(fmt.Errorf("get latest meta failed: %w", err), pkgerrors.DatabaseError)
	}
	return meta, nil
}

func (m *problemApp) ListPublishedProblems(ctx context.Context, cursorID int64, limit int) ([]repository.ProblemListItem, bool, error) {
	if limit <= 0 {
		return nil, false, pkgerrors.New(pkgerrors.InvalidParams)
	}
	items, err := m.repo.ListPublished(ctx, cursorID, limit+1)
	if err != nil {
		logx.WithContext(ctx).Errorf("list published problems failed cursor_id=%d limit=%d err=%v", cursorID, limit, err)
		return nil, false, pkgerrors.Wrap(fmt.Errorf("list published problems failed: %w", err), pkgerrors.DatabaseError)
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
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
		logx.WithContext(ctx).Errorf("create problem failed err=%v", err)
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
		logx.WithContext(ctx).Errorf("delete problem failed problem_id=%d err=%v", problemID, err)
		return pkgerrors.Wrap(fmt.Errorf("delete problem failed: %w", err), pkgerrors.ProblemDeleteFailed)
	}
	_ = m.repo.InvalidateLatestMetaCache(ctx, problemID)
	if m.statementRepo != nil {
		_ = m.statementRepo.InvalidateLatestCache(ctx, problemID)
	}
	if m.cleanupPublisher != nil {
		if err := m.cleanupPublisher.PublishProblemDeleted(ctx, problemID); err != nil {
			logx.WithContext(ctx).Errorf("publish cleanup event failed problem_id=%d err=%v", problemID, err)
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
			logx.WithContext(ctx).Errorf("check problem exists failed problem_id=%d err=%v", input.ProblemID, err)
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
		logx.WithContext(ctx).Errorf("get upload session failed problem_id=%d err=%v", input.ProblemID, err)
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
				logx.WithContext(ctx).Errorf("retry get upload session failed problem_id=%d err=%v", input.ProblemID, err2)
				return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("retry get upload session failed: %w", err2), pkgerrors.DatabaseError)
			}
		} else {
			logx.WithContext(ctx).Errorf("create upload session failed problem_id=%d err=%v", input.ProblemID, err)
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
		logx.WithContext(ctx).Errorf("get upload session failed upload_id=%d err=%v", input.UploadSessionID, err)
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
			logx.WithContext(ctx).Errorf("presign upload part failed part_number=%d err=%v", n, err)
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
		logx.WithContext(ctx).Errorf("get upload session failed upload_id=%d err=%v", input.UploadSessionID, err)
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}
	if session.ProblemID != input.ProblemID {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
	}
	if session.State == repository.UploadStateCompleted {
		meta, err := m.uploadRepo.GetProblemVersionMeta(ctx, nil, session.ProblemID, session.Version)
		if err != nil {
			logx.WithContext(ctx).Errorf("load version meta failed problem_id=%d err=%v", session.ProblemID, err)
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
		logx.WithContext(ctx).Errorf("complete multipart upload failed upload_id=%d err=%v", session.ID, err)
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("complete multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
	}

	stat, err := m.storage.StatObject(ctx, session.Bucket, session.ObjectKey)
	if err != nil {
		logx.WithContext(ctx).Errorf("stat object failed object_key=%s err=%v", session.ObjectKey, err)
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
		logx.WithContext(ctx).Errorf("complete upload persist failed upload_id=%d err=%v", session.ID, err)
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
		logx.WithContext(ctx).Errorf("get upload session failed upload_id=%d err=%v", input.UploadSessionID, err)
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
			logx.WithContext(ctx).Errorf("abort multipart upload failed upload_id=%d err=%v", session.ID, err)
			return pkgerrors.Wrap(fmt.Errorf("abort multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
		}
	}
	if err := m.uploadRepo.MarkUploadAborted(ctx, nil, session.ID); err != nil {
		logx.WithContext(ctx).Errorf("mark upload aborted failed upload_id=%d err=%v", session.ID, err)
		return pkgerrors.Wrap(fmt.Errorf("mark upload aborted failed: %w", err), pkgerrors.DatabaseError)
	}
	return nil
}

func (m *problemApp) PublishVersion(ctx context.Context, input PublishInput) error {
	if input.ProblemID <= 0 || input.Version <= 0 {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	if m.statementRepo != nil {
		exists, err := m.statementRepo.ExistsByVersion(ctx, nil, input.ProblemID, input.Version)
		if err != nil {
			logx.WithContext(ctx).Errorf("check statement exists failed problem_id=%d err=%v", input.ProblemID, err)
			return pkgerrors.Wrap(fmt.Errorf("check statement exists failed: %w", err), pkgerrors.DatabaseError)
		}
		if !exists {
			return pkgerrors.New(pkgerrors.ProblemStatementNotFound)
		}
	}
	if err := m.uploadRepo.PublishVersion(ctx, nil, input.ProblemID, input.Version); err != nil {
		if errors.Is(err, repository.ErrProblemVersionNotFound) {
			return pkgerrors.New(pkgerrors.NotFound)
		}
		if errors.Is(err, repository.ErrProblemVersionNotReady) {
			return pkgerrors.New(pkgerrors.ProblemVersionNotReadyToPublish)
		}
		logx.WithContext(ctx).Errorf("publish version failed problem_id=%d err=%v", input.ProblemID, err)
		return pkgerrors.Wrap(fmt.Errorf("publish version failed: %w", err), pkgerrors.DatabaseError)
	}
	_ = m.repo.InvalidateLatestMetaCache(ctx, input.ProblemID)
	if m.statementRepo != nil {
		_ = m.statementRepo.InvalidateLatestCache(ctx, input.ProblemID)
	}
	if m.metaPublisher != nil {
		if err := m.metaPublisher.PublishProblemMetaInvalidated(ctx, input.ProblemID, input.Version); err != nil {
			logx.WithContext(ctx).Errorf("publish problem meta invalidation failed problem_id=%d version=%d err=%v", input.ProblemID, input.Version, err)
		}
	}
	return nil
}

func (m *problemApp) GetLatestStatement(ctx context.Context, problemID int64) (repository.ProblemStatement, error) {
	if problemID <= 0 {
		return repository.ProblemStatement{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if m.statementRepo == nil {
		return repository.ProblemStatement{}, pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	statement, err := m.statementRepo.GetLatestPublished(ctx, nil, problemID)
	if err != nil {
		if errors.Is(err, repository.ErrProblemStatementNotFound) {
			return repository.ProblemStatement{}, pkgerrors.New(pkgerrors.ProblemStatementNotFound)
		}
		logx.WithContext(ctx).Errorf("get latest statement failed problem_id=%d err=%v", problemID, err)
		return repository.ProblemStatement{}, pkgerrors.Wrap(fmt.Errorf("get latest statement failed: %w", err), pkgerrors.DatabaseError)
	}
	return statement, nil
}

func (m *problemApp) GetStatementByVersion(ctx context.Context, problemID int64, version int32) (repository.ProblemStatement, error) {
	if problemID <= 0 || version <= 0 {
		return repository.ProblemStatement{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if m.statementRepo == nil {
		return repository.ProblemStatement{}, pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	statement, err := m.statementRepo.GetByVersion(ctx, nil, problemID, version)
	if err != nil {
		if errors.Is(err, repository.ErrProblemStatementNotFound) {
			return repository.ProblemStatement{}, pkgerrors.New(pkgerrors.ProblemStatementNotFound)
		}
		logx.WithContext(ctx).Errorf("get statement by version failed problem_id=%d version=%d err=%v", problemID, version, err)
		return repository.ProblemStatement{}, pkgerrors.Wrap(fmt.Errorf("get statement by version failed: %w", err), pkgerrors.DatabaseError)
	}
	return statement, nil
}

func (m *problemApp) UpdateStatement(ctx context.Context, problemID int64, version int32, statementMd string) error {
	if problemID <= 0 || version <= 0 || statementMd == "" {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	if m.statementRepo == nil {
		return pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	if m.statementMaxBytes > 0 && len(statementMd) > m.statementMaxBytes {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	meta, err := m.uploadRepo.GetProblemVersionMeta(ctx, nil, problemID, version)
	if err != nil {
		if errors.Is(err, repository.ErrProblemVersionNotFound) {
			return pkgerrors.New(pkgerrors.ProblemNotFound)
		}
		logx.WithContext(ctx).Errorf("load version meta failed problem_id=%d version=%d err=%v", problemID, version, err)
		return pkgerrors.Wrap(fmt.Errorf("load version meta failed: %w", err), pkgerrors.DatabaseError)
	}
	if meta.State != repository.ProblemVersionStateDraft {
		return pkgerrors.New(pkgerrors.ProblemStatementNotEditable)
	}
	versionID, err := m.uploadRepo.GetProblemVersionID(ctx, nil, problemID, version)
	if err != nil {
		if errors.Is(err, repository.ErrProblemVersionNotFound) {
			return pkgerrors.New(pkgerrors.ProblemNotFound)
		}
		logx.WithContext(ctx).Errorf("load version id failed problem_id=%d version=%d err=%v", problemID, version, err)
		return pkgerrors.Wrap(fmt.Errorf("load version id failed: %w", err), pkgerrors.DatabaseError)
	}
	hashBytes := sha256.Sum256([]byte(statementMd))
	statementHash := hex.EncodeToString(hashBytes[:])
	if err := m.statementRepo.Upsert(ctx, nil, repository.ProblemStatement{
		ProblemID:     problemID,
		Version:       version,
		StatementMd:   statementMd,
		StatementHash: statementHash,
	}, versionID); err != nil {
		logx.WithContext(ctx).Errorf("update statement failed problem_id=%d version=%d err=%v", problemID, version, err)
		return pkgerrors.Wrap(fmt.Errorf("update statement failed: %w", err), pkgerrors.ProblemStatementUpdateFailed)
	}
	_ = m.statementRepo.InvalidateVersionCache(ctx, problemID, version)
	_ = m.statementRepo.InvalidateLatestCache(ctx, problemID)
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
			logx.WithContext(ctx).Errorf("create multipart upload failed upload_id=%d err=%v", session.ID, err)
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("create multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
		}
		updated, err := m.uploadRepo.UpdateUploadIDIfEmpty(ctx, nil, session.ID, uploadID)
		if err != nil {
			_ = m.storage.AbortMultipartUpload(ctx, session.Bucket, session.ObjectKey, uploadID)
			logx.WithContext(ctx).Errorf("persist upload id failed upload_id=%d err=%v", session.ID, err)
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("persist upload id failed: %w", err), pkgerrors.DatabaseError)
		}
		if !updated {
			_ = m.storage.AbortMultipartUpload(ctx, session.Bucket, session.ObjectKey, uploadID)
		}
		latest, err := m.uploadRepo.GetUploadSessionByID(ctx, nil, session.ID)
		if err != nil {
			logx.WithContext(ctx).Errorf("reload upload session failed upload_id=%d err=%v", session.ID, err)
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
