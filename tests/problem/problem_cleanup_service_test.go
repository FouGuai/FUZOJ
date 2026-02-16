package problem_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"fuzoj/internal/common/db"
	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	"fuzoj/internal/problem/model"
	"fuzoj/internal/problem/repository"
	"fuzoj/internal/problem/service"
	"fuzoj/tests/testutil"
)

func TestProblemCleanupConsumerSkipExisting(t *testing.T) {
	repo := &fakeCleanupRepo{exists: true}
	store := newFakeCleanupStorage([]string{"problems/1/versions/1/data-pack.tar.zst"}, []storage.MultipartUploadInfo{
		{Key: "problems/1/versions/1/data-pack.tar.zst", UploadID: "upload-1"},
	})
	consumer := service.NewProblemCleanupConsumer(nil, repo, store, service.CleanupOptions{
		Bucket:        "problem-bucket",
		KeyPrefix:     "problems",
		BatchSize:     2,
		ListTimeout:   time.Second,
		DeleteTimeout: time.Second,
		MaxUploads:    10,
	})

	msg := buildCleanupMessage(t, 1, "problem-bucket", "problems/1/")
	err := consumer.HandleMessage(context.Background(), msg)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, store.abortedCount(), 0)
	testutil.AssertEqual(t, store.removedCount(), 0)
}

func TestProblemCleanupConsumerDeletesData(t *testing.T) {
	repo := &fakeCleanupRepo{exists: false}
	store := newFakeCleanupStorage([]string{
		"problems/2/versions/1/data-pack.tar.zst",
		"problems/2/versions/2/data-pack.tar.zst",
		"problems/2/versions/3/data-pack.tar.zst",
	}, []storage.MultipartUploadInfo{
		{Key: "problems/2/versions/4/data-pack.tar.zst", UploadID: "upload-4"},
	})
	consumer := service.NewProblemCleanupConsumer(nil, repo, store, service.CleanupOptions{
		Bucket:        "problem-bucket",
		KeyPrefix:     "problems",
		BatchSize:     2,
		ListTimeout:   time.Second,
		DeleteTimeout: time.Second,
		MaxUploads:    10,
	})

	msg := buildCleanupMessage(t, 2, "problem-bucket", "problems/2/")
	err := consumer.HandleMessage(context.Background(), msg)
	testutil.AssertNil(t, err)
	testutil.AssertEqual(t, store.abortedCount(), 1)
	testutil.AssertEqual(t, store.removedCount(), 3)
}

func buildCleanupMessage(t *testing.T, problemID int64, bucket, prefix string) *mq.Message {
	t.Helper()
	event := model.ProblemCleanupEvent{
		EventType: model.ProblemCleanupEventDeleted,
		ProblemID: problemID,
		Bucket:    bucket,
		Prefix:    prefix,
	}
	body, err := json.Marshal(event)
	testutil.AssertNil(t, err)
	return mq.NewMessage(body)
}

type fakeCleanupRepo struct {
	exists bool
}

func (r *fakeCleanupRepo) Create(ctx context.Context, tx db.Transaction, problem *repository.Problem) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *fakeCleanupRepo) Delete(ctx context.Context, tx db.Transaction, problemID int64) error {
	return errors.New("not implemented")
}

func (r *fakeCleanupRepo) Exists(ctx context.Context, tx db.Transaction, problemID int64) (bool, error) {
	return r.exists, nil
}

func (r *fakeCleanupRepo) GetLatestMeta(ctx context.Context, tx db.Transaction, problemID int64) (repository.ProblemLatestMeta, error) {
	return repository.ProblemLatestMeta{}, errors.New("not implemented")
}

func (r *fakeCleanupRepo) InvalidateLatestMetaCache(ctx context.Context, problemID int64) error {
	return nil
}

type fakeCleanupStorage struct {
	mu          sync.Mutex
	objects     []string
	uploads     []storage.MultipartUploadInfo
	removedKeys []string
	aborted     []string
}

func newFakeCleanupStorage(objects []string, uploads []storage.MultipartUploadInfo) *fakeCleanupStorage {
	return &fakeCleanupStorage{
		objects: append([]string{}, objects...),
		uploads: append([]storage.MultipartUploadInfo{}, uploads...),
	}
}

func (s *fakeCleanupStorage) GetObject(ctx context.Context, bucket, objectKey string) (storage.ObjectReader, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeCleanupStorage) PutObject(ctx context.Context, bucket, objectKey string, reader storage.ObjectReader, sizeBytes int64, contentType string) error {
	return errors.New("not implemented")
}

func (s *fakeCleanupStorage) CreateMultipartUpload(ctx context.Context, bucket, objectKey, contentType string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *fakeCleanupStorage) PresignUploadPart(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *fakeCleanupStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []storage.CompletedPart) (string, error) {
	return "", errors.New("not implemented")
}

func (s *fakeCleanupStorage) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aborted = append(s.aborted, objectKey+":"+uploadID)
	return nil
}

func (s *fakeCleanupStorage) StatObject(ctx context.Context, bucket, objectKey string) (storage.ObjectStat, error) {
	return storage.ObjectStat{}, errors.New("not implemented")
}

func (s *fakeCleanupStorage) ListObjects(ctx context.Context, bucket, prefix string) <-chan storage.ObjectInfo {
	out := make(chan storage.ObjectInfo, len(s.objects))
	go func() {
		defer close(out)
		for _, key := range s.objects {
			if strings.HasPrefix(key, prefix) {
				out <- storage.ObjectInfo{Key: key}
			}
		}
	}()
	return out
}

func (s *fakeCleanupStorage) RemoveObjects(ctx context.Context, bucket string, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removedKeys = append(s.removedKeys, keys...)
	return nil
}

func (s *fakeCleanupStorage) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (storage.ListMultipartUploadsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	uploads := make([]storage.MultipartUploadInfo, 0)
	for _, upload := range s.uploads {
		if strings.HasPrefix(upload.Key, prefix) {
			uploads = append(uploads, upload)
		}
	}
	return storage.ListMultipartUploadsResult{Uploads: uploads}, nil
}

func (s *fakeCleanupStorage) abortedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.aborted)
}

func (s *fakeCleanupStorage) removedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.removedKeys)
}
