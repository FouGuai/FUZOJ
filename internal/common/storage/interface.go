package storage

import (
	"context"
	"time"
)

// ObjectStorage defines minimal object storage operations required by the problem upload flow.
// It is intentionally small so we can swap MinIO/AWS-S3 implementations without touching business logic.
type ObjectStorage interface {
	// GetObject opens a reader for an object.
	// Caller must close the returned reader.
	GetObject(ctx context.Context, bucket, objectKey string) (ObjectReader, error)

	// PutObject uploads an object from a reader.
	// sizeBytes is the total size of the reader.
	PutObject(ctx context.Context, bucket, objectKey string, reader ObjectReader, sizeBytes int64, contentType string) error

	// CreateMultipartUpload starts a multipart upload and returns the uploadID.
	CreateMultipartUpload(ctx context.Context, bucket, objectKey, contentType string) (string, error)

	// PresignUploadPart returns a presigned URL for uploading one part via HTTP PUT.
	PresignUploadPart(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error)

	// CompleteMultipartUpload completes a multipart upload with part ETags and returns the resulting ETag if available.
	CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []CompletedPart) (string, error)

	// AbortMultipartUpload aborts an in-flight multipart upload.
	AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error

	// StatObject returns size and ETag for an object.
	StatObject(ctx context.Context, bucket, objectKey string) (ObjectStat, error)

	// ListObjects streams objects under the prefix.
	// Caller must drain the channel until it is closed.
	ListObjects(ctx context.Context, bucket, prefix string) <-chan ObjectInfo

	// RemoveObjects deletes multiple objects by key.
	RemoveObjects(ctx context.Context, bucket string, keys []string) error

	// ListMultipartUploads lists in-flight multipart uploads under the prefix.
	ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (ListMultipartUploadsResult, error)
}

// ObjectReader is a streaming reader for object data.
type ObjectReader interface {
	Read(p []byte) (int, error)
	Close() error
}

// CompletedPart describes one uploaded part for completing multipart uploads.
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// ObjectStat contains object metadata used for validation.
type ObjectStat struct {
	SizeBytes   int64
	ETag        string
	ContentType string
}

// ObjectInfo describes an object entry.
type ObjectInfo struct {
	Key       string
	SizeBytes int64
	Err       error
}

// MultipartUploadInfo describes an in-flight multipart upload.
type MultipartUploadInfo struct {
	Key      string
	UploadID string
}

// ListMultipartUploadsResult wraps multipart upload list results.
type ListMultipartUploadsResult struct {
	Uploads            []MultipartUploadInfo
	IsTruncated        bool
	NextKeyMarker      string
	NextUploadIDMarker string
}
