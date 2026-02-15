package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/db"
	"fuzoj/internal/common/storage"
	"fuzoj/internal/problem/repository"
	pkgerrors "fuzoj/pkg/errors"
)

// ProblemUploadService handles large data pack uploads via object storage.
type ProblemUploadService struct {
	dbProvider db.Provider
	metaRepo   repository.ProblemRepository
	uploadRepo repository.ProblemUploadRepository
	storage    storage.ObjectStorage

	bucket        string
	keyPrefix     string
	partSizeBytes int64
	sessionTTL    time.Duration
	presignTTL    time.Duration
}

type UploadOptions struct {
	Bucket        string
	KeyPrefix     string
	PartSizeBytes int64
	SessionTTL    time.Duration
	PresignTTL    time.Duration
}

func NewProblemUploadService(metaRepo repository.ProblemRepository, uploadRepo repository.ProblemUploadRepository, obj storage.ObjectStorage, opts UploadOptions) *ProblemUploadService {
	prefix := opts.KeyPrefix
	if prefix == "" {
		prefix = "problems"
	}
	if opts.PartSizeBytes <= 0 {
		opts.PartSizeBytes = 16 * 1024 * 1024
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 2 * time.Hour
	}
	if opts.PresignTTL <= 0 {
		opts.PresignTTL = 15 * time.Minute
	}
	return &ProblemUploadService{
		dbProvider:    nil,
		metaRepo:      metaRepo,
		uploadRepo:    uploadRepo,
		storage:       obj,
		bucket:        opts.Bucket,
		keyPrefix:     prefix,
		partSizeBytes: opts.PartSizeBytes,
		sessionTTL:    opts.SessionTTL,
		presignTTL:    opts.PresignTTL,
	}
}

func NewProblemUploadServiceWithDB(provider db.Provider, metaRepo repository.ProblemRepository, uploadRepo repository.ProblemUploadRepository, obj storage.ObjectStorage, opts UploadOptions) *ProblemUploadService {
	svc := NewProblemUploadService(metaRepo, uploadRepo, obj, opts)
	svc.dbProvider = provider
	return svc
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

func (s *ProblemUploadService) PrepareDataPackUpload(ctx context.Context, input PrepareUploadInput) (PrepareUploadOutput, error) {
	if input.ProblemID <= 0 || input.IdempotencyKey == "" {
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if s.metaRepo != nil {
		exists, err := s.metaRepo.Exists(ctx, nil, input.ProblemID)
		if err != nil {
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("check problem exists failed: %w", err), pkgerrors.DatabaseError)
		}
		if !exists {
			return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemNotFound)
		}
	}
	if s.storage == nil {
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ServiceUnavailable)
	}
	if s.bucket == "" {
		return PrepareUploadOutput{}, pkgerrors.Wrap(errors.New("bucket is empty"), pkgerrors.InternalServerError)
	}
	contentType := input.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	now := time.Now()
	expiresAt := now.Add(s.sessionTTL)

	// Fast-path: reuse existing session for idempotency key.
	existing, err := s.uploadRepo.GetUploadSessionByIdempotencyKey(ctx, nil, input.ProblemID, input.IdempotencyKey)
	if err == nil {
		if existing.State != repository.UploadStateUploading {
			return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
		}
		if now.After(existing.ExpiresAt) {
			_ = s.uploadRepo.MarkUploadExpired(ctx, nil, existing.ID)
			return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadExpired)
		}
		if existing.UploadID == "" {
			return s.ensureMultipartUpload(ctx, existing, contentType)
		}
		return PrepareUploadOutput{
			UploadSessionID:   existing.ID,
			ProblemID:         existing.ProblemID,
			Version:           existing.Version,
			Bucket:            existing.Bucket,
			ObjectKey:         existing.ObjectKey,
			MultipartUploadID: existing.UploadID,
			PartSizeBytes:     s.partSizeBytes,
			ExpiresAt:         existing.ExpiresAt,
		}, nil
	} else if !errors.Is(err, repository.ErrUploadNotFound) {
		return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}

	// Create a new upload session with allocated version in one transaction.
	var created repository.UploadSession
	if err := s.withTx(ctx, func(tx repositoryTx) error {
		// Re-check within tx to prevent race for same idempotency key.
		ex, err := s.uploadRepo.GetUploadSessionByIdempotencyKey(ctx, tx, input.ProblemID, input.IdempotencyKey)
		if err == nil {
			created = ex
			return nil
		} else if !errors.Is(err, repository.ErrUploadNotFound) {
			return err
		}

		version, err := s.uploadRepo.AllocateNextVersion(ctx, tx, input.ProblemID)
		if err != nil {
			return err
		}
		objectKey := s.dataPackObjectKey(input.ProblemID, version)

		session, err := s.uploadRepo.CreateUploadSession(ctx, tx, repository.CreateUploadSessionInput{
			ProblemID:         input.ProblemID,
			Version:           version,
			IdempotencyKey:    input.IdempotencyKey,
			Bucket:            s.bucket,
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
		created = session
		return nil
	}); err != nil {
		// Duplicate key conflicts are treated as idempotent retries; other errors bubble up.
		if key, ok := repositoryUniqueViolation(err); ok && key == "pdu_problem_idem_uq" {
			ex, err2 := s.uploadRepo.GetUploadSessionByIdempotencyKey(ctx, nil, input.ProblemID, input.IdempotencyKey)
			if err2 == nil {
				created = ex
			} else {
				return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("retry get upload session failed: %w", err2), pkgerrors.DatabaseError)
			}
		} else {
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("create upload session failed: %w", err), pkgerrors.DatabaseError)
		}
	}

	return s.ensureMultipartUpload(ctx, created, contentType)
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

func (s *ProblemUploadService) SignUploadParts(ctx context.Context, input SignPartsInput) (SignPartsOutput, error) {
	if input.ProblemID <= 0 || input.UploadSessionID <= 0 || len(input.PartNumbers) == 0 {
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	session, err := s.uploadRepo.GetUploadSessionByID(ctx, nil, input.UploadSessionID)
	if err != nil {
		if errors.Is(err, repository.ErrUploadNotFound) {
			return SignPartsOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
		}
		return SignPartsOutput{}, pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}
	if session.ProblemID != input.ProblemID {
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
	}
	if session.State != repository.UploadStateUploading {
		return SignPartsOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.uploadRepo.MarkUploadExpired(ctx, nil, session.ID)
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
		u, err := s.storage.PresignUploadPart(ctx, session.Bucket, session.ObjectKey, session.UploadID, n, s.presignTTL, session.ContentType)
		if err != nil {
			return SignPartsOutput{}, pkgerrors.Wrap(fmt.Errorf("presign upload part failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
		}
		urls[n] = u
	}
	return SignPartsOutput{
		URLs:             urls,
		ExpiresInSeconds: int64(s.presignTTL.Seconds()),
	}, nil
}

type CompletedPartInput struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
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

func (s *ProblemUploadService) CompleteDataPackUpload(ctx context.Context, input CompleteUploadInput) (CompleteUploadOutput, error) {
	if input.ProblemID <= 0 || input.UploadSessionID <= 0 || len(input.Parts) == 0 {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if len(input.ManifestJSON) == 0 || len(input.ConfigJSON) == 0 {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}
	if input.ManifestHash == "" || input.DataPackHash == "" {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.InvalidParams)
	}

	session, err := s.uploadRepo.GetUploadSessionByID(ctx, nil, input.UploadSessionID)
	if err != nil {
		if errors.Is(err, repository.ErrUploadNotFound) {
			return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
		}
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}
	if session.ProblemID != input.ProblemID {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadNotFound)
	}
	if session.State == repository.UploadStateCompleted {
		meta, err := s.uploadRepo.GetProblemVersionMeta(ctx, nil, session.ProblemID, session.Version)
		if err != nil {
			return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("load version meta failed: %w", err), pkgerrors.DatabaseError)
		}
		// Idempotent completion: return already persisted values.
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
		_ = s.uploadRepo.MarkUploadExpired(ctx, nil, session.ID)
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

	if _, err := s.storage.CompleteMultipartUpload(ctx, session.Bucket, session.ObjectKey, session.UploadID, parts); err != nil {
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("complete multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
	}

	stat, err := s.storage.StatObject(ctx, session.Bucket, session.ObjectKey)
	if err != nil {
		return CompleteUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("stat object failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
	}
	if session.ExpectedSizeBytes > 0 && stat.SizeBytes != session.ExpectedSizeBytes {
		return CompleteUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadConflict)
	}

	if err := s.withTx(ctx, func(tx repositoryTx) error {
		versionID, err := s.uploadRepo.GetProblemVersionID(ctx, tx, session.ProblemID, session.Version)
		if err != nil {
			return err
		}
		if err := s.uploadRepo.UpdateProblemVersionDraftMeta(ctx, tx, session.ProblemID, session.Version, input.ConfigJSON, input.ManifestHash, session.ObjectKey, input.DataPackHash); err != nil {
			return err
		}
		if err := s.uploadRepo.UpsertManifest(ctx, tx, versionID, input.ManifestJSON); err != nil {
			return err
		}
		if err := s.uploadRepo.UpsertDataPack(ctx, tx, versionID, session.ObjectKey, stat.SizeBytes, "", input.DataPackHash); err != nil {
			return err
		}
		if err := s.uploadRepo.MarkUploadCompleted(ctx, tx, session.ID); err != nil {
			return err
		}
		return nil
	}); err != nil {
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

type AbortUploadInput struct {
	ProblemID       int64
	UploadSessionID int64
}

func (s *ProblemUploadService) AbortDataPackUpload(ctx context.Context, input AbortUploadInput) error {
	if input.ProblemID <= 0 || input.UploadSessionID <= 0 {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	session, err := s.uploadRepo.GetUploadSessionByID(ctx, nil, input.UploadSessionID)
	if err != nil {
		if errors.Is(err, repository.ErrUploadNotFound) {
			return pkgerrors.New(pkgerrors.ProblemUploadNotFound)
		}
		return pkgerrors.Wrap(fmt.Errorf("get upload session failed: %w", err), pkgerrors.DatabaseError)
	}
	if session.ProblemID != input.ProblemID {
		return pkgerrors.New(pkgerrors.ProblemUploadNotFound)
	}
	if session.State == repository.UploadStateCompleted {
		return pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}
	if session.UploadID != "" {
		if err := s.storage.AbortMultipartUpload(ctx, session.Bucket, session.ObjectKey, session.UploadID); err != nil {
			return pkgerrors.Wrap(fmt.Errorf("abort multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
		}
	}
	if err := s.uploadRepo.MarkUploadAborted(ctx, nil, session.ID); err != nil {
		return pkgerrors.Wrap(fmt.Errorf("mark upload aborted failed: %w", err), pkgerrors.DatabaseError)
	}
	return nil
}

type PublishInput struct {
	ProblemID int64
	Version   int32
}

func (s *ProblemUploadService) PublishVersion(ctx context.Context, input PublishInput) error {
	if input.ProblemID <= 0 || input.Version <= 0 {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	if err := s.uploadRepo.PublishVersion(ctx, nil, input.ProblemID, input.Version); err != nil {
		if errors.Is(err, repository.ErrProblemVersionNotFound) {
			return pkgerrors.New(pkgerrors.NotFound)
		}
		if errors.Is(err, repository.ErrProblemVersionNotReady) {
			return pkgerrors.New(pkgerrors.ProblemVersionNotReadyToPublish)
		}
		return pkgerrors.Wrap(fmt.Errorf("publish version failed: %w", err), pkgerrors.DatabaseError)
	}
	_ = s.metaRepo.InvalidateLatestMetaCache(ctx, input.ProblemID)
	return nil
}

func (s *ProblemUploadService) ensureMultipartUpload(ctx context.Context, session repository.UploadSession, contentType string) (PrepareUploadOutput, error) {
	if session.State != repository.UploadStateUploading {
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadStateInvalid)
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.uploadRepo.MarkUploadExpired(ctx, nil, session.ID)
		return PrepareUploadOutput{}, pkgerrors.New(pkgerrors.ProblemUploadExpired)
	}

	if session.UploadID == "" {
		uploadID, err := s.storage.CreateMultipartUpload(ctx, session.Bucket, session.ObjectKey, contentType)
		if err != nil {
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("create multipart upload failed: %w", err), pkgerrors.ProblemUploadObjectStorageFailed)
		}
		updated, err := s.uploadRepo.UpdateUploadIDIfEmpty(ctx, nil, session.ID, uploadID)
		if err != nil {
			_ = s.storage.AbortMultipartUpload(ctx, session.Bucket, session.ObjectKey, uploadID)
			return PrepareUploadOutput{}, pkgerrors.Wrap(fmt.Errorf("persist upload id failed: %w", err), pkgerrors.DatabaseError)
		}
		if !updated {
			// Another request has set upload_id; abort ours to avoid leaking multipart uploads.
			_ = s.storage.AbortMultipartUpload(ctx, session.Bucket, session.ObjectKey, uploadID)
		}
		// Re-fetch latest session to return authoritative upload_id.
		latest, err := s.uploadRepo.GetUploadSessionByID(ctx, nil, session.ID)
		if err != nil {
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
		PartSizeBytes:     s.partSizeBytes,
		ExpiresAt:         session.ExpiresAt,
	}, nil
}

func (s *ProblemUploadService) dataPackObjectKey(problemID int64, version int32) string {
	return fmt.Sprintf("%s/%d/versions/%d/data-pack.tar.zst", s.keyPrefix, problemID, version)
}

// repositoryTx is the minimal interface we need for transactions across repositories.
type repositoryTx interface {
	db.Transaction
}

func (s *ProblemUploadService) withTx(ctx context.Context, fn func(tx repositoryTx) error) error {
	database, err := db.CurrentDatabase(s.dbProvider)
	if err != nil {
		// Should not happen in production. Keep behavior safe and explicit.
		return errors.New("database is nil")
	}
	return database.Transaction(ctx, func(tx db.Transaction) error {
		return fn(tx)
	})
}

func repositoryUniqueViolation(err error) (string, bool) {
	return db.UniqueViolation(err)
}
