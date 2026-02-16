package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	"fuzoj/internal/problem/model"
	"fuzoj/internal/problem/repository"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

const (
	defaultCleanupBatchSize   = 1000
	defaultCleanupListTimeout = 30 * time.Second
	defaultCleanupDeleteTTL   = 2 * time.Minute
	defaultCleanupMaxUploads  = 1000
)

// CleanupOptions controls cleanup behavior.
type CleanupOptions struct {
	Bucket        string
	KeyPrefix     string
	BatchSize     int
	ListTimeout   time.Duration
	DeleteTimeout time.Duration
	MaxUploads    int
}

// ProblemCleanupConsumer handles cleanup events for deleted problems.
type ProblemCleanupConsumer struct {
	mqClient      mq.MessageQueue
	repo          repository.ProblemRepository
	storage       storage.ObjectStorage
	bucket        string
	keyPrefix     string
	batchSize     int
	listTimeout   time.Duration
	deleteTimeout time.Duration
	maxUploads    int
}

// NewProblemCleanupConsumer creates a cleanup consumer.
func NewProblemCleanupConsumer(mqClient mq.MessageQueue, repo repository.ProblemRepository, obj storage.ObjectStorage, opts CleanupOptions) *ProblemCleanupConsumer {
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
	return &ProblemCleanupConsumer{
		mqClient:      mqClient,
		repo:          repo,
		storage:       obj,
		bucket:        opts.Bucket,
		keyPrefix:     opts.KeyPrefix,
		batchSize:     batchSize,
		listTimeout:   listTimeout,
		deleteTimeout: deleteTimeout,
		maxUploads:    maxUploads,
	}
}

// Subscribe registers the cleanup handler and starts consuming.
func (c *ProblemCleanupConsumer) Subscribe(ctx context.Context, topic, consumerGroup string, opts *mq.SubscribeOptions) error {
	if c == nil || c.mqClient == nil {
		return errors.New("message queue is nil")
	}
	if topic == "" {
		return errors.New("cleanup topic is required")
	}
	options := opts
	if options == nil {
		options = &mq.SubscribeOptions{}
	}
	if options.ConsumerGroup == "" {
		options.ConsumerGroup = consumerGroup
	}
	if err := c.mqClient.SubscribeWithOptions(ctx, topic, c.handleMessage, options); err != nil {
		return err
	}
	return c.mqClient.Start()
}

// HandleMessage processes a cleanup event message.
func (c *ProblemCleanupConsumer) HandleMessage(ctx context.Context, message *mq.Message) error {
	return c.handleMessage(ctx, message)
}

func (c *ProblemCleanupConsumer) handleMessage(ctx context.Context, message *mq.Message) error {
	var event model.ProblemCleanupEvent
	if err := json.Unmarshal(message.Body, &event); err != nil {
		logger.Warn(ctx, "parse cleanup event failed", zap.Error(err))
		return nil
	}
	if event.EventType != model.ProblemCleanupEventDeleted {
		return nil
	}
	if event.ProblemID <= 0 {
		logger.Warn(ctx, "cleanup event missing problem_id")
		return nil
	}

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
			logger.Info(ctx, "skip cleanup for existing problem", zap.Int64("problem_id", event.ProblemID))
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
