package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOConfig holds object storage settings for MinIO.
type MinIOConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
	UseSSL    bool   `yaml:"useSSL"`
	Bucket    string `yaml:"bucket"`

	// PresignTTL controls default presigned URL lifetime.
	PresignTTL time.Duration `yaml:"presignTTL"`
}

// MinIOStorage implements ObjectStorage using MinIO S3-compatible APIs.
type MinIOStorage struct {
	core *minio.Core
}

func NewMinIOStorage(cfg MinIOConfig) (*MinIOStorage, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint is required")
	}
	if cfg.AccessKey == "" {
		return nil, fmt.Errorf("minio accessKey is required")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("minio secretKey is required")
	}
	core, err := minio.NewCore(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio core failed: %w", err)
	}
	return &MinIOStorage{core: core}, nil
}

func (s *MinIOStorage) CreateMultipartUpload(ctx context.Context, bucket, objectKey, contentType string) (string, error) {
	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}

	uploadID, err := s.core.NewMultipartUpload(ctx, bucket, objectKey, opts)
	if err != nil {
		return "", fmt.Errorf("minio new multipart upload failed: %w", err)
	}
	return uploadID, nil
}

func (s *MinIOStorage) GetObject(ctx context.Context, bucket, objectKey string) (ObjectReader, error) {
	obj, _, _, err := s.core.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio get object failed: %w", err)
	}
	return obj, nil
}

func (s *MinIOStorage) PutObject(ctx context.Context, bucket, objectKey string, reader ObjectReader, sizeBytes int64, contentType string) error {
	if reader == nil {
		return fmt.Errorf("reader is required")
	}
	if objectKey == "" {
		return fmt.Errorf("objectKey is required")
	}
	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}
	_, err := s.core.PutObject(ctx, bucket, objectKey, reader, sizeBytes, "", "", opts)
	if err != nil {
		return fmt.Errorf("minio put object failed: %w", err)
	}
	return nil
}

func (s *MinIOStorage) PresignUploadPart(ctx context.Context, bucket, objectKey, uploadID string, partNumber int, ttl time.Duration, contentType string) (string, error) {
	if uploadID == "" {
		return "", fmt.Errorf("uploadID is required")
	}
	if partNumber <= 0 {
		return "", fmt.Errorf("partNumber must be positive")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	// Presign a PUT request to: /{bucket}/{objectKey}?partNumber=N&uploadId=...
	// We intentionally do not sign Content-Type to keep browser/CLI compatibility simpler.
	reqParams := make(url.Values)
	reqParams.Set("partNumber", fmt.Sprintf("%d", partNumber))
	reqParams.Set("uploadId", uploadID)

	u, err := s.core.Presign(ctx, "PUT", bucket, objectKey, ttl, reqParams)
	if err != nil {
		return "", fmt.Errorf("minio presign upload part failed: %w", err)
	}
	return u.String(), nil
}

func (s *MinIOStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []CompletedPart) (string, error) {
	if uploadID == "" {
		return "", fmt.Errorf("uploadID is required")
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("parts are required")
	}

	completeParts := make([]minio.CompletePart, 0, len(parts))
	for _, p := range parts {
		if p.PartNumber <= 0 || p.ETag == "" {
			return "", fmt.Errorf("invalid completed part")
		}
		completeParts = append(completeParts, minio.CompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		})
	}

	info, err := s.core.CompleteMultipartUpload(ctx, bucket, objectKey, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("minio complete multipart upload failed: %w", err)
	}
	return info.ETag, nil
}

func (s *MinIOStorage) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	if uploadID == "" {
		return fmt.Errorf("uploadID is required")
	}
	if err := s.core.AbortMultipartUpload(ctx, bucket, objectKey, uploadID); err != nil {
		return fmt.Errorf("minio abort multipart upload failed: %w", err)
	}
	return nil
}

func (s *MinIOStorage) StatObject(ctx context.Context, bucket, objectKey string) (ObjectStat, error) {
	info, err := s.core.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return ObjectStat{}, fmt.Errorf("minio stat object failed: %w", err)
	}
	return ObjectStat{
		SizeBytes:   info.Size,
		ETag:        info.ETag,
		ContentType: info.ContentType,
	}, nil
}

func (s *MinIOStorage) ListObjects(ctx context.Context, bucket, prefix string) <-chan ObjectInfo {
	out := make(chan ObjectInfo, 1)
	if s.core == nil {
		out <- ObjectInfo{Err: fmt.Errorf("minio core is nil")}
		close(out)
		return out
	}

	objCh := s.core.Client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	go func() {
		defer close(out)
		for obj := range objCh {
			if obj.Err != nil {
				out <- ObjectInfo{Err: fmt.Errorf("minio list objects failed: %w", obj.Err)}
				continue
			}
			out <- ObjectInfo{Key: obj.Key, SizeBytes: obj.Size}
		}
	}()
	return out
}

func (s *MinIOStorage) RemoveObjects(ctx context.Context, bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if s.core == nil {
		return fmt.Errorf("minio core is nil")
	}
	objCh := make(chan minio.ObjectInfo, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		objCh <- minio.ObjectInfo{Key: key}
	}
	close(objCh)

	errCh := s.core.RemoveObjects(ctx, bucket, objCh, minio.RemoveObjectsOptions{})
	for err := range errCh {
		if err.Err != nil {
			return fmt.Errorf("minio remove object failed: %w", err.Err)
		}
	}
	return nil
}

func (s *MinIOStorage) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (ListMultipartUploadsResult, error) {
	if s.core == nil {
		return ListMultipartUploadsResult{}, fmt.Errorf("minio core is nil")
	}
	if maxUploads <= 0 {
		maxUploads = 1000
	}
	res, err := s.core.ListMultipartUploads(ctx, bucket, prefix, keyMarker, uploadIDMarker, "", maxUploads)
	if err != nil {
		return ListMultipartUploadsResult{}, fmt.Errorf("minio list multipart uploads failed: %w", err)
	}
	uploads := make([]MultipartUploadInfo, 0, len(res.Uploads))
	for _, upload := range res.Uploads {
		uploads = append(uploads, MultipartUploadInfo{
			Key:      upload.Key,
			UploadID: upload.UploadID,
		})
	}
	return ListMultipartUploadsResult{
		Uploads:            uploads,
		IsTruncated:        res.IsTruncated,
		NextKeyMarker:      res.NextKeyMarker,
		NextUploadIDMarker: res.NextUploadIDMarker,
	}, nil
}
