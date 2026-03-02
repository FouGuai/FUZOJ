package tests

import (
	"context"
	"database/sql"
	"io"
	"strings"
	"testing"
	"time"

	"fuzoj/internal/common/storage"
	"fuzoj/services/submit_service/internal/repository"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type fakeLogStorage struct {
	putCalls []string
	getCalls []string
	data     map[string]string
}

func (f *fakeLogStorage) GetObject(ctx context.Context, bucket, objectKey string) (storage.ObjectReader, error) {
	f.getCalls = append(f.getCalls, bucket+"/"+objectKey)
	content := ""
	if f.data != nil {
		content = f.data[objectKey]
	}
	return &fakeReader{data: strings.NewReader(content)}, nil
}

func (f *fakeLogStorage) PutObject(ctx context.Context, bucket, objectKey string, reader storage.ObjectReader, sizeBytes int64, contentType string) error {
	f.putCalls = append(f.putCalls, bucket+"/"+objectKey)
	if f.data == nil {
		f.data = make(map[string]string)
	}
	payload, _ := ioReadAll(reader)
	f.data[objectKey] = payload
	return nil
}

func (f *fakeLogStorage) CreateMultipartUpload(ctx context.Context, bucket, objectKey, contentType string) (string, error) {
	return "", nil
}

func (f *fakeLogStorage) PresignUploadPart(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
	return "", nil
}

func (f *fakeLogStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []storage.CompletedPart) (string, error) {
	return "", nil
}

func (f *fakeLogStorage) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	return nil
}

func (f *fakeLogStorage) StatObject(ctx context.Context, bucket, objectKey string) (storage.ObjectStat, error) {
	return storage.ObjectStat{}, nil
}

func (f *fakeLogStorage) ListObjects(ctx context.Context, bucket, prefix string) <-chan storage.ObjectInfo {
	ch := make(chan storage.ObjectInfo)
	close(ch)
	return ch
}

func (f *fakeLogStorage) RemoveObjects(ctx context.Context, bucket string, keys []string) error {
	return nil
}

func (f *fakeLogStorage) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (storage.ListMultipartUploadsResult, error) {
	return storage.ListMultipartUploadsResult{}, nil
}

type fakeLogConn struct{}

func (f *fakeLogConn) Exec(query string, args ...any) (sql.Result, error) {
	return fakeLogResult{}, nil
}

func (f *fakeLogConn) ExecCtx(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return fakeLogResult{}, nil
}

func (f *fakeLogConn) Prepare(query string) (sqlx.StmtSession, error) {
	return nil, nil
}

func (f *fakeLogConn) PrepareCtx(ctx context.Context, query string) (sqlx.StmtSession, error) {
	return nil, nil
}

func (f *fakeLogConn) QueryRow(v any, query string, args ...any) error {
	return f.QueryRowCtx(context.Background(), v, query, args...)
}

func (f *fakeLogConn) QueryRowCtx(ctx context.Context, v any, query string, args ...any) error {
	if logItem, ok := v.(*repository.SubmissionLog); ok {
		logItem.SubmissionID = "sub-2"
		logItem.LogType = repository.LogTypeRuntime
		logItem.TestID = "1"
		logItem.LogPath = "logs/sub-2/runtime_log/1.log"
		logItem.LogSize = int64(len([]byte("large-content")))
		return nil
	}
	return nil
}

func (f *fakeLogConn) QueryRowPartial(v any, query string, args ...any) error {
	return f.QueryRowCtx(context.Background(), v, query, args...)
}

func (f *fakeLogConn) QueryRowPartialCtx(ctx context.Context, v any, query string, args ...any) error {
	return f.QueryRowCtx(ctx, v, query, args...)
}

func (f *fakeLogConn) QueryRows(v any, query string, args ...any) error {
	return f.QueryRowsCtx(context.Background(), v, query, args...)
}

func (f *fakeLogConn) QueryRowsCtx(ctx context.Context, v any, query string, args ...any) error {
	return nil
}

func (f *fakeLogConn) QueryRowsPartial(v any, query string, args ...any) error {
	return f.QueryRowsCtx(context.Background(), v, query, args...)
}

func (f *fakeLogConn) QueryRowsPartialCtx(ctx context.Context, v any, query string, args ...any) error {
	return f.QueryRowsCtx(ctx, v, query, args...)
}

func (f *fakeLogConn) RawDB() (*sql.DB, error) {
	return nil, nil
}

func (f *fakeLogConn) Transact(fn func(sqlx.Session) error) error {
	return fn(f)
}

func (f *fakeLogConn) TransactCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error {
	return fn(ctx, f)
}

type fakeLogResult struct{}

func (f fakeLogResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (f fakeLogResult) RowsAffected() (int64, error) {
	return 1, nil
}

type fakeReader struct {
	data *strings.Reader
}

func (f *fakeReader) Read(p []byte) (int, error) {
	return f.data.Read(p)
}

func (f *fakeReader) Close() error {
	return nil
}

func ioReadAll(reader storage.ObjectReader) (string, error) {
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 256)
	for {
		n, err := reader.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}
	return string(buf), nil
}

func TestSubmissionLogRepository_SaveAndGetSmallLog(t *testing.T) {
	_, redisClient := newTestRedis(t)
	storageClient := &fakeLogStorage{}
	repo := repository.NewSubmissionLogRepository(&fakeLogConn{}, redisClient, storageClient, "log-bucket", "logs", 1024, time.Minute)

	record := repository.LogRecord{
		SubmissionID: "sub-1",
		LogType:      repository.LogTypeCompileLog,
		Content:      "hello",
	}
	if err := repo.Save(context.Background(), record); err != nil {
		t.Fatalf("save log failed: %v", err)
	}
	logItem, err := repo.Get(context.Background(), "sub-1", repository.LogTypeCompileLog, "")
	if err != nil {
		t.Fatalf("get log failed: %v", err)
	}
	if logItem.Content != "hello" {
		t.Fatalf("unexpected log content: %s", logItem.Content)
	}
	if len(storageClient.putCalls) != 0 {
		t.Fatalf("unexpected storage put calls: %d", len(storageClient.putCalls))
	}
}

func TestSubmissionLogRepository_SaveAndGetLargeLog(t *testing.T) {
	_, redisClient := newTestRedis(t)
	storageClient := &fakeLogStorage{}
	repo := repository.NewSubmissionLogRepository(&fakeLogConn{}, redisClient, storageClient, "log-bucket", "logs", 4, time.Minute)

	record := repository.LogRecord{
		SubmissionID: "sub-2",
		LogType:      repository.LogTypeRuntime,
		TestID:       "1",
		Content:      "large-content",
	}
	if err := repo.Save(context.Background(), record); err != nil {
		t.Fatalf("save log failed: %v", err)
	}
	logItem, err := repo.Get(context.Background(), "sub-2", repository.LogTypeRuntime, "1")
	if err != nil {
		t.Fatalf("get log failed: %v", err)
	}
	if logItem.Content != "large-content" {
		t.Fatalf("unexpected log content: %s", logItem.Content)
	}
	if len(storageClient.putCalls) != 1 {
		t.Fatalf("expected storage put call")
	}
	if len(storageClient.getCalls) == 0 {
		t.Fatalf("expected storage get call")
	}
}
