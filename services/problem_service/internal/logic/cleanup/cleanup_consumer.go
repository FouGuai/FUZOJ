package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/storage"
	"fuzoj/internal/problem/model"
	"fuzoj/services/problem_service/internal/repository"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultCleanupBatchSize   = 1000
	defaultCleanupListTimeout = 30 * time.Second
	defaultCleanupDeleteTTL   = 2 * time.Minute
	defaultCleanupMaxUploads  = 1000
)

// CleanupOptions controls cleanup behavior.
type CleanupOptions struct {
	Bucket          string
	KeyPrefix       string
	BatchSize       int
	ListTimeout     time.Duration
	DeleteTimeout   time.Duration
	MaxUploads      int
	MaxRetries      int
	RetryDelay      time.Duration
	MessageTTL      time.Duration
	DeadLetterTopic string
}

// ProblemCleanupConsumer handles cleanup events for deleted problems.
type ProblemCleanupConsumer struct {
	repo             repository.ProblemRepository
	storage          storage.ObjectStorage
	bucket           string
	keyPrefix        string
	batchSize        int
	listTimeout      time.Duration
	deleteTimeout    time.Duration
	maxUploads       int
	maxRetries       int
	retryDelay       time.Duration
	messageTTL       time.Duration
	deadLetterTopic  string
	deadLetterPusher *kq.Pusher
}

// NewProblemCleanupConsumer creates a cleanup consumer.
func NewProblemCleanupConsumer(repo repository.ProblemRepository, obj storage.ObjectStorage, opts CleanupOptions) *ProblemCleanupConsumer {
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = defaultCleanupBatchSize
	}
	listTimeout := opts.ListTimeout
	if listTimeout <= 0 {
		listTimeout = defaultCleanupListTimeout
	}
	deleteTimeout := opts.DeleteTimeout
	if deleteTimeout <= 0 {
		deleteTimeout = defaultCleanupDeleteTTL
	}
	maxUploads := opts.MaxUploads
	if maxUploads <= 0 {
		maxUploads = defaultCleanupMaxUploads
	}
	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryDelay := opts.RetryDelay
	if retryDelay <= 0 {
		retryDelay = time.Second
	}
	return &ProblemCleanupConsumer{
		repo:            repo,
		storage:         obj,
		bucket:          opts.Bucket,
		keyPrefix:       opts.KeyPrefix,
		batchSize:       batchSize,
		listTimeout:     listTimeout,
		deleteTimeout:   deleteTimeout,
		maxUploads:      maxUploads,
		maxRetries:      maxRetries,
		retryDelay:      retryDelay,
		messageTTL:      opts.MessageTTL,
		deadLetterTopic: opts.DeadLetterTopic,
	}
}

// SetDeadLetterPusher configures the dead-letter topic pusher.
func (c *ProblemCleanupConsumer) SetDeadLetterPusher(pusher *kq.Pusher) {
	c.deadLetterPusher = pusher
}

// Consume processes a cleanup event message.
func (c *ProblemCleanupConsumer) Consume(ctx context.Context, key, value string) error {
	if value == "" {
		return nil
	}
	var event model.ProblemCleanupEvent
	if err := json.Unmarshal([]byte(value), &event); err != nil {
		logx.WithContext(ctx).Errorf("parse cleanup event failed: %v", err)
		return nil
	}
	if event.EventType != model.ProblemCleanupEventDeleted {
		return nil
	}
	if event.ProblemID <= 0 {
		logx.WithContext(ctx).Error("cleanup event missing problem_id")
		return nil
	}
	if c.messageTTL > 0 && !event.RequestedAt.IsZero() && time.Since(event.RequestedAt) > c.messageTTL {
		return nil
	}

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := c.handleEvent(ctx, event); err == nil {
			return nil
		} else if attempt >= c.maxRetries {
			if c.deadLetterTopic != "" && c.deadLetterPusher != nil {
				_ = c.deadLetterPusher.PushWithKey(ctx, key, value)
			}
			logx.WithContext(ctx).Errorf("cleanup event failed after retries: %v", err)
			return nil
		}
		time.Sleep(c.retryDelay)
	}
	return nil
}

func (c *ProblemCleanupConsumer) handleEvent(ctx context.Context, event model.ProblemCleanupEvent) error {
	bucket := event.Bucket
	if bucket == "" {
		bucket = c.bucket
	}
	prefix := event.Prefix
	if prefix == "" {
		prefix = problemObjectPrefix(c.keyPrefix, event.ProblemID)
	}
	if bucket == "" || prefix == "" {
		return errors.New("cleanup bucket or prefix is empty")
	}
	if c.storage == nil {
		return errors.New("object storage is nil")
	}
	if c.repo != nil {
		exists, err := c.repo.Exists(ctx, nil, event.ProblemID)
		if err != nil {
			return fmt.Errorf("check problem exists failed: %w", err)
		}
		if exists {
			logx.WithContext(ctx).Infof("skip cleanup for existing problem: %d", event.ProblemID)
			return nil
		}
	}
	if err := c.abortMultipartUploads(ctx, bucket, prefix); err != nil {
		return err
	}
	if err := c.removeObjectsByPrefix(ctx, bucket, prefix); err != nil {
		return err
	}
	return nil
}

func (c *ProblemCleanupConsumer) abortMultipartUploads(ctx context.Context, bucket, prefix string) error {
	var keyMarker string
	var uploadIDMarker string
	for {
		listCtx, cancel := context.WithTimeout(ctx, c.listTimeout)
		res, err := c.storage.ListMultipartUploads(listCtx, bucket, prefix, keyMarker, uploadIDMarker, c.maxUploads)
		cancel()
		if err != nil {
			return err
		}
		for _, upload := range res.Uploads {
			delCtx, delCancel := context.WithTimeout(ctx, c.deleteTimeout)
			err := c.storage.AbortMultipartUpload(delCtx, bucket, upload.Key, upload.UploadID)
			delCancel()
			if err != nil {
				return fmt.Errorf("abort multipart upload failed: %w", err)
			}
		}
		if !res.IsTruncated {
			return nil
		}
		keyMarker = res.NextKeyMarker
		uploadIDMarker = res.NextUploadIDMarker
	}
}

func (c *ProblemCleanupConsumer) removeObjectsByPrefix(ctx context.Context, bucket, prefix string) error {
	listCtx, cancel := context.WithTimeout(ctx, c.listTimeout)
	objCh := c.storage.ListObjects(listCtx, bucket, prefix)
	defer cancel()

	batch := make([]string, 0, c.batchSize)
	for obj := range objCh {
		if obj.Err != nil {
			return obj.Err
		}
		if obj.Key == "" {
			continue
		}
		batch = append(batch, obj.Key)
		if len(batch) >= c.batchSize {
			if err := c.removeBatch(ctx, bucket, batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if err := c.removeBatch(ctx, bucket, batch); err != nil {
			return err
		}
	}
	if err := listCtx.Err(); err != nil {
		return fmt.Errorf("list objects failed: %w", err)
	}
	return nil
}

func (c *ProblemCleanupConsumer) removeBatch(ctx context.Context, bucket string, keys []string) error {
	delCtx, cancel := context.WithTimeout(ctx, c.deleteTimeout)
	defer cancel()
	if err := c.storage.RemoveObjects(delCtx, bucket, keys); err != nil {
		return fmt.Errorf("remove objects failed: %w", err)
	}
	return nil
}
