package tests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"fuzoj/internal/common/db"
	"fuzoj/internal/common/storage"
	"fuzoj/internal/problem/repository"
	"fuzoj/internal/problem/service"
)

func TestProblemUploadServiceFullFlow(t *testing.T) {
	testCases := []struct {
		name          string
		idemKey       string
		expectedHash  string
		manifestHash  string
		dataPackHash  string
		expectedParts []int
	}{
		{
			name:          "full flow",
			idemKey:       "idem-1",
			expectedHash:  "hash-1",
			manifestHash:  "manifest-hash",
			dataPackHash:  "hash-1",
			expectedParts: []int{1, 2},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			uploadRepo := newFakeUploadRepo()
			metaRepo := &fakeProblemRepo{}
			fakeStorage := newFakeObjectStorage()
			fakeDB := &fakeDB{}

			svc := service.NewProblemUploadServiceWithDB(db.NewStaticProvider(fakeDB), metaRepo, uploadRepo, fakeStorage, service.UploadOptions{
				Bucket:        "problem-bucket",
				KeyPrefix:     "problems",
				PartSizeBytes: 8 * 1024 * 1024,
				SessionTTL:    time.Hour,
				PresignTTL:    time.Minute,
			})

			prepareOut, err := svc.PrepareDataPackUpload(ctx, service.PrepareUploadInput{
				ProblemID:      1,
				IdempotencyKey: tc.idemKey,
				ExpectedSHA256: tc.expectedHash,
				ContentType:    "application/octet-stream",
				CreatedBy:      10,
			})
			AssertNil(t, err)
			AssertTrue(t, prepareOut.UploadSessionID > 0, "upload session should be created")
			AssertTrue(t, prepareOut.MultipartUploadID != "", "multipart upload id should be set")

			signOut, err := svc.SignUploadParts(ctx, service.SignPartsInput{
				ProblemID:       1,
				UploadSessionID: prepareOut.UploadSessionID,
				PartNumbers:     tc.expectedParts,
			})
			AssertNil(t, err)
			AssertEqual(t, len(signOut.URLs), len(tc.expectedParts))
			AssertEqual(t, signOut.ExpiresInSeconds, int64(time.Minute.Seconds()))

			manifest := MustMarshalJSON(t, map[string]interface{}{"name": "demo"})
			config := MustMarshalJSON(t, map[string]interface{}{"time_limit_ms": 1000})

			parts := make([]service.CompletedPartInput, 0, len(tc.expectedParts))
			for _, partNumber := range tc.expectedParts {
				parts = append(parts, service.CompletedPartInput{
					PartNumber: partNumber,
					ETag:       fmt.Sprintf("etag-%d", partNumber),
				})
			}

			completeOut, err := svc.CompleteDataPackUpload(ctx, service.CompleteUploadInput{
				ProblemID:       1,
				UploadSessionID: prepareOut.UploadSessionID,
				Parts:           parts,
				ManifestJSON:    manifest,
				ConfigJSON:      config,
				ManifestHash:    tc.manifestHash,
				DataPackHash:    tc.dataPackHash,
			})
			AssertNil(t, err)
			AssertEqual(t, completeOut.Version, prepareOut.Version)
			AssertEqual(t, completeOut.DataPackKey, prepareOut.ObjectKey)

			err = svc.PublishVersion(ctx, service.PublishInput{
				ProblemID: 1,
				Version:   prepareOut.Version,
			})
			AssertNil(t, err)
		})
	}
}

type fakeProblemRepo struct {
}

func (f *fakeProblemRepo) Create(ctx context.Context, tx db.Transaction, problem *repository.Problem) (int64, error) {
	return 0, errors.New("not implemented")
}

func (f *fakeProblemRepo) Delete(ctx context.Context, tx db.Transaction, problemID int64) error {
	return errors.New("not implemented")
}

func (f *fakeProblemRepo) Exists(ctx context.Context, tx db.Transaction, problemID int64) (bool, error) {
	if problemID <= 0 {
		return false, nil
	}
	return problemID == 1, nil
}

func (f *fakeProblemRepo) GetLatestMeta(ctx context.Context, tx db.Transaction, problemID int64) (repository.ProblemLatestMeta, error) {
	return repository.ProblemLatestMeta{}, errors.New("not implemented")
}

func (f *fakeProblemRepo) InvalidateLatestMetaCache(ctx context.Context, problemID int64) error {
	return nil
}

type fakeUploadRepo struct {
	mu                 sync.Mutex
	nextVersion        map[int64]int32
	nextSessionID      int64
	sessions           map[int64]repository.UploadSession
	sessionByIdemKey   map[string]int64
	versionMeta        map[string]repository.ProblemVersionMeta
	versionID          map[string]int64
	nextVersionID      int64
	completedSessionID map[int64]bool
}

func newFakeUploadRepo() *fakeUploadRepo {
	return &fakeUploadRepo{
		nextVersion:        make(map[int64]int32),
		nextSessionID:      1,
		sessions:           make(map[int64]repository.UploadSession),
		sessionByIdemKey:   make(map[string]int64),
		versionMeta:        make(map[string]repository.ProblemVersionMeta),
		versionID:          make(map[string]int64),
		nextVersionID:      1,
		completedSessionID: make(map[int64]bool),
	}
}

func (r *fakeUploadRepo) AllocateNextVersion(ctx context.Context, tx db.Transaction, problemID int64) (int32, error) {
	if problemID <= 0 {
		return 0, errors.New("problemID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.nextVersion[problemID]
	if next == 0 {
		next = 1
	}
	r.nextVersion[problemID] = next + 1
	return next, nil
}

func (r *fakeUploadRepo) GetUploadSessionByIdempotencyKey(ctx context.Context, tx db.Transaction, problemID int64, idempotencyKey string) (repository.UploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%d:%s", problemID, idempotencyKey)
	id, ok := r.sessionByIdemKey[key]
	if !ok {
		return repository.UploadSession{}, repository.ErrUploadNotFound
	}
	return r.sessions[id], nil
}

func (r *fakeUploadRepo) CreateUploadSession(ctx context.Context, tx db.Transaction, input repository.CreateUploadSessionInput) (repository.UploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if input.ProblemID <= 0 || input.Version <= 0 || input.IdempotencyKey == "" {
		return repository.UploadSession{}, errors.New("invalid input")
	}
	if input.Bucket == "" || input.ObjectKey == "" {
		return repository.UploadSession{}, errors.New("bucket and objectKey are required")
	}

	id := r.nextSessionID
	r.nextSessionID++

	now := time.Now()
	session := repository.UploadSession{
		ID:                id,
		ProblemID:         input.ProblemID,
		Version:           input.Version,
		IdempotencyKey:    input.IdempotencyKey,
		Bucket:            input.Bucket,
		ObjectKey:         input.ObjectKey,
		UploadID:          "",
		ExpectedSizeBytes: input.ExpectedSizeBytes,
		ExpectedSHA256:    input.ExpectedSHA256,
		ContentType:       input.ContentType,
		State:             repository.UploadStateUploading,
		ExpiresAt:         input.ExpiresAt,
		CreatedBy:         input.CreatedBy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	r.sessions[id] = session
	key := fmt.Sprintf("%d:%s", input.ProblemID, input.IdempotencyKey)
	r.sessionByIdemKey[key] = id
	return session, nil
}

func (r *fakeUploadRepo) GetUploadSessionByID(ctx context.Context, tx db.Transaction, uploadSessionID int64) (repository.UploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[uploadSessionID]
	if !ok {
		return repository.UploadSession{}, repository.ErrUploadNotFound
	}
	return session, nil
}

func (r *fakeUploadRepo) UpdateUploadIDIfEmpty(ctx context.Context, tx db.Transaction, uploadSessionID int64, uploadID string) (bool, error) {
	if uploadID == "" {
		return false, errors.New("uploadID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[uploadSessionID]
	if !ok {
		return false, repository.ErrUploadNotFound
	}
	if session.UploadID != "" {
		return false, nil
	}
	session.UploadID = uploadID
	session.UpdatedAt = time.Now()
	r.sessions[uploadSessionID] = session
	return true, nil
}

func (r *fakeUploadRepo) MarkUploadCompleted(ctx context.Context, tx db.Transaction, uploadSessionID int64) error {
	return r.updateState(uploadSessionID, repository.UploadStateCompleted)
}

func (r *fakeUploadRepo) MarkUploadAborted(ctx context.Context, tx db.Transaction, uploadSessionID int64) error {
	return r.updateState(uploadSessionID, repository.UploadStateAborted)
}

func (r *fakeUploadRepo) MarkUploadExpired(ctx context.Context, tx db.Transaction, uploadSessionID int64) error {
	return r.updateState(uploadSessionID, repository.UploadStateExpired)
}

func (r *fakeUploadRepo) updateState(uploadSessionID int64, state int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[uploadSessionID]
	if !ok {
		return repository.ErrUploadNotFound
	}
	session.State = state
	session.UpdatedAt = time.Now()
	r.sessions[uploadSessionID] = session
	if state == repository.UploadStateCompleted {
		r.completedSessionID[uploadSessionID] = true
	}
	return nil
}

func (r *fakeUploadRepo) GetProblemVersionMeta(ctx context.Context, tx db.Transaction, problemID int64, version int32) (repository.ProblemVersionMeta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%d:%d", problemID, version)
	meta, ok := r.versionMeta[key]
	if !ok {
		return repository.ProblemVersionMeta{}, repository.ErrProblemVersionNotFound
	}
	return meta, nil
}

func (r *fakeUploadRepo) GetProblemVersionID(ctx context.Context, tx db.Transaction, problemID int64, version int32) (int64, error) {
	if problemID <= 0 || version <= 0 {
		return 0, errors.New("invalid version")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%d:%d", problemID, version)
	if id, ok := r.versionID[key]; ok {
		return id, nil
	}
	id := r.nextVersionID
	r.nextVersionID++
	r.versionID[key] = id
	return id, nil
}

func (r *fakeUploadRepo) UpdateProblemVersionDraftMeta(ctx context.Context, tx db.Transaction, problemID int64, version int32, configJSON []byte, manifestHash, dataPackKey, dataPackHash string) error {
	if problemID <= 0 || version <= 0 {
		return errors.New("invalid version")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%d:%d", problemID, version)
	r.versionMeta[key] = repository.ProblemVersionMeta{
		ProblemID:    problemID,
		Version:      version,
		State:        0,
		ManifestHash: manifestHash,
		DataPackKey:  dataPackKey,
		DataPackHash: dataPackHash,
	}
	return nil
}

func (r *fakeUploadRepo) UpsertManifest(ctx context.Context, tx db.Transaction, problemVersionID int64, manifestJSON []byte) error {
	if problemVersionID <= 0 {
		return errors.New("problemVersionID is required")
	}
	return nil
}

func (r *fakeUploadRepo) UpsertDataPack(ctx context.Context, tx db.Transaction, problemVersionID int64, objectKey string, sizeBytes int64, md5, sha256 string) error {
	if problemVersionID <= 0 {
		return errors.New("problemVersionID is required")
	}
	if objectKey == "" {
		return errors.New("objectKey is required")
	}
	return nil
}

func (r *fakeUploadRepo) PublishVersion(ctx context.Context, tx db.Transaction, problemID int64, version int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%d:%d", problemID, version)
	meta, ok := r.versionMeta[key]
	if !ok {
		return repository.ErrProblemVersionNotFound
	}
	if meta.ManifestHash == "" || meta.DataPackHash == "" || meta.DataPackKey == "" {
		return repository.ErrProblemVersionNotReady
	}
	meta.State = 1
	r.versionMeta[key] = meta
	return nil
}

type fakeObjectStorage struct {
	mu           sync.Mutex
	uploads      map[string]string
	objectStats  map[string]storage.ObjectStat
	abortedCount int
}

func newFakeObjectStorage() *fakeObjectStorage {
	return &fakeObjectStorage{
		uploads:     make(map[string]string),
		objectStats: make(map[string]storage.ObjectStat),
	}
}

func (s *fakeObjectStorage) CreateMultipartUpload(ctx context.Context, bucket, objectKey, contentType string) (string, error) {
	if bucket == "" || objectKey == "" {
		return "", errors.New("bucket and objectKey are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s/%s", bucket, objectKey)
	uploadID := fmt.Sprintf("upload-%s", objectKey)
	s.uploads[key] = uploadID
	return uploadID, nil
}

func (s *fakeObjectStorage) GetObject(ctx context.Context, bucket, objectKey string) (storage.ObjectReader, error) {
	return &fakeObjectReader{Reader: bytes.NewReader([]byte{})}, nil
}

func (s *fakeObjectStorage) PutObject(ctx context.Context, bucket, objectKey string, reader storage.ObjectReader, sizeBytes int64, contentType string) error {
	if bucket == "" || objectKey == "" {
		return errors.New("bucket and objectKey are required")
	}
	return nil
}

func (s *fakeObjectStorage) PresignUploadPart(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
	if uploadID == "" || partNumber <= 0 {
		return "", errors.New("invalid upload part request")
	}
	return fmt.Sprintf("https://example.invalid/%s/%d", uploadID, partNumber), nil
}

func (s *fakeObjectStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []storage.CompletedPart) (string, error) {
	if uploadID == "" {
		return "", errors.New("uploadID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s/%s", bucket, objectKey)
	if s.uploads[key] != uploadID {
		return "", errors.New("upload session not found")
	}
	s.objectStats[key] = storage.ObjectStat{
		SizeBytes:   0,
		ETag:        "etag",
		ContentType: "application/octet-stream",
	}
	return "etag", nil
}

func (s *fakeObjectStorage) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s/%s", bucket, objectKey)
	delete(s.uploads, key)
	s.abortedCount++
	return nil
}

func (s *fakeObjectStorage) StatObject(ctx context.Context, bucket, objectKey string) (storage.ObjectStat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s/%s", bucket, objectKey)
	stat, ok := s.objectStats[key]
	if !ok {
		return storage.ObjectStat{}, errors.New("object not found")
	}
	return stat, nil
}

type fakeObjectReader struct {
	*bytes.Reader
}

func (r *fakeObjectReader) Close() error {
	return nil
}

type fakeDB struct {
}

func (f *fakeDB) Query(ctx context.Context, query string, args ...interface{}) (db.Rows, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeDB) QueryRow(ctx context.Context, query string, args ...interface{}) db.Row {
	return fakeRow{}
}

func (f *fakeDB) Exec(ctx context.Context, query string, args ...interface{}) (db.Result, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeDB) Transaction(ctx context.Context, fn func(tx db.Transaction) error) error {
	return fn(&fakeTx{})
}

func (f *fakeDB) BeginTx(ctx context.Context, opts *db.TxOptions) (db.Transaction, error) {
	return &fakeTx{}, nil
}

func (f *fakeDB) Prepare(ctx context.Context, query string) (db.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeDB) Ping(ctx context.Context) error {
	return nil
}

func (f *fakeDB) Close() error {
	return nil
}

func (f *fakeDB) Stats() db.Stats {
	return db.Stats{}
}

func (f *fakeDB) GetDB() interface{} {
	return nil
}

type fakeTx struct {
}

func (f *fakeTx) Query(ctx context.Context, query string, args ...interface{}) (db.Rows, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTx) QueryRow(ctx context.Context, query string, args ...interface{}) db.Row {
	return fakeRow{}
}

func (f *fakeTx) Exec(ctx context.Context, query string, args ...interface{}) (db.Result, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTx) Prepare(ctx context.Context, query string) (db.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTx) Commit() error {
	return nil
}

func (f *fakeTx) Rollback() error {
	return nil
}

type fakeRow struct {
}

func (f fakeRow) Scan(dest ...interface{}) error {
	return errors.New("not implemented")
}
