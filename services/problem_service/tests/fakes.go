package tests

import (
	"context"
	"errors"
	"time"

	"fuzoj/internal/common/storage"
	"fuzoj/services/problem_service/internal/repository"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type fakeProblemRepo struct {
	createFn          func(ctx context.Context, session sqlx.Session, problem *repository.Problem) (int64, error)
	deleteFn          func(ctx context.Context, session sqlx.Session, problemID int64) error
	existsFn          func(ctx context.Context, session sqlx.Session, problemID int64) (bool, error)
	listPublishedFn   func(ctx context.Context, cursorID int64, limit int) ([]repository.ProblemListItem, error)
	getLatestMetaFn   func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemLatestMeta, error)
	invalidateCacheFn func(ctx context.Context, problemID int64) error
}

func (f *fakeProblemRepo) Create(ctx context.Context, session sqlx.Session, problem *repository.Problem) (int64, error) {
	if f.createFn == nil {
		return 0, errors.New("create not implemented")
	}
	return f.createFn(ctx, session, problem)
}

func (f *fakeProblemRepo) Delete(ctx context.Context, session sqlx.Session, problemID int64) error {
	if f.deleteFn == nil {
		return errors.New("delete not implemented")
	}
	return f.deleteFn(ctx, session, problemID)
}

func (f *fakeProblemRepo) Exists(ctx context.Context, session sqlx.Session, problemID int64) (bool, error) {
	if f.existsFn == nil {
		return false, errors.New("exists not implemented")
	}
	return f.existsFn(ctx, session, problemID)
}

func (f *fakeProblemRepo) ListPublished(ctx context.Context, cursorID int64, limit int) ([]repository.ProblemListItem, error) {
	if f.listPublishedFn == nil {
		return nil, errors.New("list published not implemented")
	}
	return f.listPublishedFn(ctx, cursorID, limit)
}

func (f *fakeProblemRepo) GetLatestMeta(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemLatestMeta, error) {
	if f.getLatestMetaFn == nil {
		return repository.ProblemLatestMeta{}, errors.New("get latest meta not implemented")
	}
	return f.getLatestMetaFn(ctx, session, problemID)
}

func (f *fakeProblemRepo) InvalidateLatestMetaCache(ctx context.Context, problemID int64) error {
	if f.invalidateCacheFn == nil {
		return nil
	}
	return f.invalidateCacheFn(ctx, problemID)
}

type fakeStatementRepo struct {
	getLatestFn         func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemStatement, error)
	getByVersionFn      func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemStatement, error)
	existsFn            func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error)
	upsertFn            func(ctx context.Context, session sqlx.Session, statement repository.ProblemStatement, problemVersionID int64) error
	invalidateLatestFn  func(ctx context.Context, problemID int64) error
	invalidateVersionFn func(ctx context.Context, problemID int64, version int32) error
}

func (f *fakeStatementRepo) GetLatestPublished(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemStatement, error) {
	if f.getLatestFn == nil {
		return repository.ProblemStatement{}, repository.ErrProblemStatementNotFound
	}
	return f.getLatestFn(ctx, session, problemID)
}

func (f *fakeStatementRepo) GetByVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemStatement, error) {
	if f.getByVersionFn == nil {
		return repository.ProblemStatement{}, repository.ErrProblemStatementNotFound
	}
	return f.getByVersionFn(ctx, session, problemID, version)
}

func (f *fakeStatementRepo) ExistsByVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) (bool, error) {
	if f.existsFn == nil {
		return false, nil
	}
	return f.existsFn(ctx, session, problemID, version)
}

func (f *fakeStatementRepo) Upsert(ctx context.Context, session sqlx.Session, statement repository.ProblemStatement, problemVersionID int64) error {
	if f.upsertFn == nil {
		return errors.New("upsert statement not implemented")
	}
	return f.upsertFn(ctx, session, statement, problemVersionID)
}

func (f *fakeStatementRepo) InvalidateLatestCache(ctx context.Context, problemID int64) error {
	if f.invalidateLatestFn == nil {
		return nil
	}
	return f.invalidateLatestFn(ctx, problemID)
}

func (f *fakeStatementRepo) InvalidateVersionCache(ctx context.Context, problemID int64, version int32) error {
	if f.invalidateVersionFn == nil {
		return nil
	}
	return f.invalidateVersionFn(ctx, problemID, version)
}

type fakeUploadRepo struct {
	allocateNextVersionFn    func(ctx context.Context, session sqlx.Session, problemID int64) (int32, error)
	getSessionByIdemFn       func(ctx context.Context, session sqlx.Session, problemID int64, idempotencyKey string) (repository.UploadSession, error)
	createSessionFn          func(ctx context.Context, session sqlx.Session, input repository.CreateUploadSessionInput) (repository.UploadSession, error)
	getSessionByIDFn         func(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error)
	updateUploadIDIfEmptyFn  func(ctx context.Context, session sqlx.Session, uploadSessionID int64, uploadID string) (bool, error)
	markCompletedFn          func(ctx context.Context, session sqlx.Session, uploadSessionID int64) error
	markAbortedFn            func(ctx context.Context, session sqlx.Session, uploadSessionID int64) error
	markExpiredFn            func(ctx context.Context, session sqlx.Session, uploadSessionID int64) error
	getProblemVersionMetaFn  func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemVersionMeta, error)
	getProblemVersionIDFn    func(ctx context.Context, session sqlx.Session, problemID int64, version int32) (int64, error)
	updateProblemDraftMetaFn func(ctx context.Context, session sqlx.Session, problemID int64, version int32, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) error
	upsertManifestFn         func(ctx context.Context, session sqlx.Session, problemVersionID int64, manifestJSON []byte) error
	upsertDataPackFn         func(ctx context.Context, session sqlx.Session, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error
	publishVersionFn         func(ctx context.Context, session sqlx.Session, problemID int64, version int32) error
}

func (f *fakeUploadRepo) AllocateNextVersion(ctx context.Context, session sqlx.Session, problemID int64) (int32, error) {
	if f.allocateNextVersionFn == nil {
		return 0, errors.New("allocate next version not implemented")
	}
	return f.allocateNextVersionFn(ctx, session, problemID)
}

func (f *fakeUploadRepo) GetUploadSessionByIdempotencyKey(ctx context.Context, session sqlx.Session, problemID int64, idempotencyKey string) (repository.UploadSession, error) {
	if f.getSessionByIdemFn == nil {
		return repository.UploadSession{}, repository.ErrUploadNotFound
	}
	return f.getSessionByIdemFn(ctx, session, problemID, idempotencyKey)
}

func (f *fakeUploadRepo) CreateUploadSession(ctx context.Context, session sqlx.Session, input repository.CreateUploadSessionInput) (repository.UploadSession, error) {
	if f.createSessionFn == nil {
		return repository.UploadSession{}, errors.New("create session not implemented")
	}
	return f.createSessionFn(ctx, session, input)
}

func (f *fakeUploadRepo) GetUploadSessionByID(ctx context.Context, session sqlx.Session, uploadSessionID int64) (repository.UploadSession, error) {
	if f.getSessionByIDFn == nil {
		return repository.UploadSession{}, repository.ErrUploadNotFound
	}
	return f.getSessionByIDFn(ctx, session, uploadSessionID)
}

func (f *fakeUploadRepo) UpdateUploadIDIfEmpty(ctx context.Context, session sqlx.Session, uploadSessionID int64, uploadID string) (bool, error) {
	if f.updateUploadIDIfEmptyFn == nil {
		return false, errors.New("update upload id not implemented")
	}
	return f.updateUploadIDIfEmptyFn(ctx, session, uploadSessionID, uploadID)
}

func (f *fakeUploadRepo) MarkUploadCompleted(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
	if f.markCompletedFn == nil {
		return errors.New("mark completed not implemented")
	}
	return f.markCompletedFn(ctx, session, uploadSessionID)
}

func (f *fakeUploadRepo) MarkUploadAborted(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
	if f.markAbortedFn == nil {
		return errors.New("mark aborted not implemented")
	}
	return f.markAbortedFn(ctx, session, uploadSessionID)
}

func (f *fakeUploadRepo) MarkUploadExpired(ctx context.Context, session sqlx.Session, uploadSessionID int64) error {
	if f.markExpiredFn == nil {
		return errors.New("mark expired not implemented")
	}
	return f.markExpiredFn(ctx, session, uploadSessionID)
}

func (f *fakeUploadRepo) GetProblemVersionMeta(ctx context.Context, session sqlx.Session, problemID int64, version int32) (repository.ProblemVersionMeta, error) {
	if f.getProblemVersionMetaFn == nil {
		return repository.ProblemVersionMeta{}, repository.ErrProblemVersionNotFound
	}
	return f.getProblemVersionMetaFn(ctx, session, problemID, version)
}

func (f *fakeUploadRepo) GetProblemVersionID(ctx context.Context, session sqlx.Session, problemID int64, version int32) (int64, error) {
	if f.getProblemVersionIDFn == nil {
		return 0, repository.ErrProblemVersionNotFound
	}
	return f.getProblemVersionIDFn(ctx, session, problemID, version)
}

func (f *fakeUploadRepo) UpdateProblemVersionDraftMeta(ctx context.Context, session sqlx.Session, problemID int64, version int32, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) error {
	if f.updateProblemDraftMetaFn == nil {
		return errors.New("update draft meta not implemented")
	}
	return f.updateProblemDraftMetaFn(ctx, session, problemID, version, configJSON, manifestHash, dataPackKey, dataPackHash)
}

func (f *fakeUploadRepo) UpsertManifest(ctx context.Context, session sqlx.Session, problemVersionID int64, manifestJSON []byte) error {
	if f.upsertManifestFn == nil {
		return errors.New("upsert manifest not implemented")
	}
	return f.upsertManifestFn(ctx, session, problemVersionID, manifestJSON)
}

func (f *fakeUploadRepo) UpsertDataPack(ctx context.Context, session sqlx.Session, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error {
	if f.upsertDataPackFn == nil {
		return errors.New("upsert data pack not implemented")
	}
	return f.upsertDataPackFn(ctx, session, problemVersionID, objectKey, sizeBytes, md5, sha256)
}

func (f *fakeUploadRepo) PublishVersion(ctx context.Context, session sqlx.Session, problemID int64, version int32) error {
	if f.publishVersionFn == nil {
		return errors.New("publish version not implemented")
	}
	return f.publishVersionFn(ctx, session, problemID, version)
}

type fakeStorage struct {
	createMultipartUploadFn func(ctx context.Context, bucket, objectKey, contentType string) (string, error)
	presignUploadPartFn     func(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error)
	completeMultipartFn     func(ctx context.Context, bucket, objectKey, uploadID string, parts []storage.CompletedPart) (string, error)
	abortMultipartFn        func(ctx context.Context, bucket, objectKey, uploadID string) error
	statObjectFn            func(ctx context.Context, bucket, objectKey string) (storage.ObjectStat, error)
	listObjectsFn           func(ctx context.Context, bucket, prefix string) <-chan storage.ObjectInfo
	removeObjectsFn         func(ctx context.Context, bucket string, keys []string) error
	listMultipartUploadsFn  func(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (storage.ListMultipartUploadsResult, error)
}

func (f *fakeStorage) GetObject(ctx context.Context, bucket, objectKey string) (storage.ObjectReader, error) {
	return nil, errors.New("get object not implemented")
}

func (f *fakeStorage) PutObject(ctx context.Context, bucket, objectKey string, reader storage.ObjectReader, sizeBytes int64, contentType string) error {
	return errors.New("put object not implemented")
}

func (f *fakeStorage) CreateMultipartUpload(ctx context.Context, bucket, objectKey, contentType string) (string, error) {
	if f.createMultipartUploadFn == nil {
		return "", errors.New("create multipart upload not implemented")
	}
	return f.createMultipartUploadFn(ctx, bucket, objectKey, contentType)
}

func (f *fakeStorage) PresignUploadPart(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
	if f.presignUploadPartFn == nil {
		return "", errors.New("presign upload part not implemented")
	}
	return f.presignUploadPartFn(ctx, bucket, objectKey, uploadID, partNumber, ttl, contentType)
}

func (f *fakeStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []storage.CompletedPart) (string, error) {
	if f.completeMultipartFn == nil {
		return "", errors.New("complete multipart upload not implemented")
	}
	return f.completeMultipartFn(ctx, bucket, objectKey, uploadID, parts)
}

func (f *fakeStorage) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	if f.abortMultipartFn == nil {
		return errors.New("abort multipart upload not implemented")
	}
	return f.abortMultipartFn(ctx, bucket, objectKey, uploadID)
}

func (f *fakeStorage) StatObject(ctx context.Context, bucket, objectKey string) (storage.ObjectStat, error) {
	if f.statObjectFn == nil {
		return storage.ObjectStat{}, errors.New("stat object not implemented")
	}
	return f.statObjectFn(ctx, bucket, objectKey)
}

func (f *fakeStorage) ListObjects(ctx context.Context, bucket, prefix string) <-chan storage.ObjectInfo {
	if f.listObjectsFn == nil {
		ch := make(chan storage.ObjectInfo)
		close(ch)
		return ch
	}
	return f.listObjectsFn(ctx, bucket, prefix)
}

func (f *fakeStorage) RemoveObjects(ctx context.Context, bucket string, keys []string) error {
	if f.removeObjectsFn == nil {
		return errors.New("remove objects not implemented")
	}
	return f.removeObjectsFn(ctx, bucket, keys)
}

func (f *fakeStorage) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (storage.ListMultipartUploadsResult, error) {
	if f.listMultipartUploadsFn == nil {
		return storage.ListMultipartUploadsResult{}, errors.New("list multipart uploads not implemented")
	}
	return f.listMultipartUploadsFn(ctx, bucket, prefix, keyMarker, uploadIDMarker, maxUploads)
}

type fakeCleanupPublisher struct {
	publishFn func(ctx context.Context, problemID int64) error
}

func (f *fakeCleanupPublisher) PublishProblemDeleted(ctx context.Context, problemID int64) error {
	if f.publishFn == nil {
		return nil
	}
	return f.publishFn(ctx, problemID)
}

type fakeMetaPublisher struct {
	publishFn func(ctx context.Context, problemID int64, version int32) error
}

func (f *fakeMetaPublisher) PublishProblemMetaInvalidated(ctx context.Context, problemID int64, version int32) error {
	if f.publishFn == nil {
		return nil
	}
	return f.publishFn(ctx, problemID, version)
}

func (f *fakeMetaPublisher) Close() error {
	return nil
}
