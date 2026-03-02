package tests

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"time"

	"fuzoj/internal/common/storage"
	"fuzoj/services/submit_service/internal/model"
	"fuzoj/services/submit_service/internal/repository"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type fakeSubmissionRepo struct {
	createFn  func(ctx context.Context, session sqlx.Session, submission *repository.Submission) error
	getByIDFn func(ctx context.Context, session sqlx.Session, submissionID string) (*repository.Submission, error)
}

func (f *fakeSubmissionRepo) Create(ctx context.Context, session sqlx.Session, submission *repository.Submission) error {
	if f.createFn == nil {
		return errors.New("create not implemented")
	}
	return f.createFn(ctx, session, submission)
}

func (f *fakeSubmissionRepo) GetByID(ctx context.Context, session sqlx.Session, submissionID string) (*repository.Submission, error) {
	if f.getByIDFn == nil {
		return nil, repository.ErrSubmissionNotFound
	}
	return f.getByIDFn(ctx, session, submissionID)
}

type fakeSubmissionsModel struct {
	finalStatus         map[string]string
	insertFn            func(ctx context.Context, data *model.Submissions) (sql.Result, error)
	findOneFn           func(ctx context.Context, submissionID string) (*model.Submissions, error)
	updateFn            func(ctx context.Context, data *model.Submissions) error
	deleteFn            func(ctx context.Context, submissionID string) error
	updateFinalStatusFn func(ctx context.Context, submissionID, payload string, finishedAt time.Time) (sql.Result, error)
}

func (f *fakeSubmissionsModel) Insert(ctx context.Context, data *model.Submissions) (sql.Result, error) {
	if f.insertFn == nil {
		return fakeSQLResult{rows: 1}, nil
	}
	return f.insertFn(ctx, data)
}

func (f *fakeSubmissionsModel) FindOne(ctx context.Context, submissionID string) (*model.Submissions, error) {
	if f.findOneFn == nil {
		return nil, model.ErrNotFound
	}
	return f.findOneFn(ctx, submissionID)
}

func (f *fakeSubmissionsModel) Update(ctx context.Context, data *model.Submissions) error {
	if f.updateFn == nil {
		return nil
	}
	return f.updateFn(ctx, data)
}

func (f *fakeSubmissionsModel) Delete(ctx context.Context, submissionID string) error {
	if f.deleteFn == nil {
		return nil
	}
	return f.deleteFn(ctx, submissionID)
}

func (f *fakeSubmissionsModel) WithSession(session sqlx.Session) model.SubmissionsModel {
	return f
}

func (f *fakeSubmissionsModel) FindFinalStatus(ctx context.Context, submissionID string) (string, error) {
	if f.finalStatus == nil {
		return "", model.ErrNotFound
	}
	payload, ok := f.finalStatus[submissionID]
	if !ok {
		return "", model.ErrNotFound
	}
	return payload, nil
}

func (f *fakeSubmissionsModel) FindFinalStatusBatch(ctx context.Context, submissionIDs []string) ([]model.SubmissionFinalStatus, error) {
	if f.finalStatus == nil || len(submissionIDs) == 0 {
		return nil, nil
	}
	rows := make([]model.SubmissionFinalStatus, 0, len(submissionIDs))
	for _, id := range submissionIDs {
		payload, ok := f.finalStatus[id]
		if !ok {
			continue
		}
		rows = append(rows, model.SubmissionFinalStatus{SubmissionID: id, FinalStatus: payload})
	}
	return rows, nil
}

func (f *fakeSubmissionsModel) UpdateFinalStatus(ctx context.Context, submissionID string, payload string, finishedAt time.Time) (sql.Result, error) {
	if f.updateFinalStatusFn != nil {
		return f.updateFinalStatusFn(ctx, submissionID, payload, finishedAt)
	}
	return fakeSQLResult{rows: 1}, nil
}

type fakeSQLResult struct {
	rows int64
	last int64
}

func (r fakeSQLResult) LastInsertId() (int64, error) {
	return r.last, nil
}

func (r fakeSQLResult) RowsAffected() (int64, error) {
	return r.rows, nil
}

type fakeStorage struct {
	putObjectFn func(ctx context.Context, bucket, objectKey string, reader storage.ObjectReader, sizeBytes int64, contentType string) error
}

func (f *fakeStorage) GetObject(ctx context.Context, bucket, objectKey string) (storage.ObjectReader, error) {
	return nil, errors.New("get object not implemented")
}

func (f *fakeStorage) PutObject(ctx context.Context, bucket, objectKey string, reader storage.ObjectReader, sizeBytes int64, contentType string) error {
	if f.putObjectFn == nil {
		return errors.New("put object not implemented")
	}
	return f.putObjectFn(ctx, bucket, objectKey, reader, sizeBytes, contentType)
}

func (f *fakeStorage) CreateMultipartUpload(ctx context.Context, bucket, objectKey, contentType string) (string, error) {
	return "", errors.New("create multipart upload not implemented")
}

func (f *fakeStorage) PresignUploadPart(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
	return "", errors.New("presign upload part not implemented")
}

func (f *fakeStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []storage.CompletedPart) (string, error) {
	return "", errors.New("complete multipart upload not implemented")
}

func (f *fakeStorage) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	return errors.New("abort multipart upload not implemented")
}

func (f *fakeStorage) StatObject(ctx context.Context, bucket, objectKey string) (storage.ObjectStat, error) {
	return storage.ObjectStat{}, errors.New("stat object not implemented")
}

func (f *fakeStorage) ListObjects(ctx context.Context, bucket, prefix string) <-chan storage.ObjectInfo {
	ch := make(chan storage.ObjectInfo)
	close(ch)
	return ch
}

func (f *fakeStorage) RemoveObjects(ctx context.Context, bucket string, keys []string) error {
	return errors.New("remove objects not implemented")
}

func (f *fakeStorage) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (storage.ListMultipartUploadsResult, error) {
	return storage.ListMultipartUploadsResult{}, errors.New("list multipart uploads not implemented")
}

type fakePusher struct {
	pushFn  func(ctx context.Context, key, value string) error
	closeFn func() error

	keys   []string
	values []string
}

func (f *fakePusher) PushWithKey(ctx context.Context, key, value string) error {
	f.keys = append(f.keys, key)
	f.values = append(f.values, value)
	if f.pushFn == nil {
		return nil
	}
	return f.pushFn(ctx, key, value)
}

func (f *fakePusher) Close() error {
	if f.closeFn == nil {
		return nil
	}
	return f.closeFn()
}

func drainReader(reader io.Reader) {
	_, _ = io.Copy(io.Discard, reader)
}
