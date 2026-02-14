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
