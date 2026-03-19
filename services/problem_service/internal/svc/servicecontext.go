// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"context"
	"database/sql"

	"fuzoj/internal/common/storage"
	"fuzoj/pkg/problem/metapubsub"
	"fuzoj/services/problem_service/internal/config"
	"fuzoj/services/problem_service/internal/logic/cleanup"
	"fuzoj/services/problem_service/internal/metainvalidation"
	"fuzoj/services/problem_service/internal/repository"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/syncx"
)

type ServiceContext struct {
	Config           config.Config
	Conn             sqlx.SqlConn
	Cache            cache.Cache
	ProblemRepo      repository.ProblemRepository
	StatementRepo    repository.ProblemStatementRepository
	UploadRepo       repository.ProblemUploadRepository
	Storage          storage.ObjectStorage
	CleanupQueue     queue.MessageQueue
	CleanupConsumer  *cleanup.ProblemCleanupConsumer
	CleanupPublisher *cleanup.ProblemCleanupPublisher
	MetaPublisher    MetaPublisher
	DeadLetterPusher *kq.Pusher
}

type MetaPublisher interface {
	PublishProblemMetaInvalidated(ctx context.Context, problemID int64, version int32) error
	Close() error
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	var cacheClient cache.Cache
	if len(c.Cache) > 0 {
		cacheClient = cache.New(c.Cache, syncx.NewSingleFlight(), cache.NewStat("problem"), sql.ErrNoRows)
	}

	var storageClient storage.ObjectStorage
	if c.MinIO.Endpoint != "" {
		st, err := storage.NewMinIOStorage(storage.MinIOConfig{
			Endpoint:  c.MinIO.Endpoint,
			AccessKey: c.MinIO.AccessKey,
			SecretKey: c.MinIO.SecretKey,
			UseSSL:    c.MinIO.UseSSL,
		})
		if err == nil {
			storageClient = st
		} else {
			logx.Errorf("init minio storage failed: %v", err)
		}
	}

	problemRepo := repository.NewProblemRepository(conn, cacheClient)
	var statementLocal *repository.StatementLocalCache
	if c.Statement.LocalCacheSize > 0 {
		statementLocal = repository.NewStatementLocalCache(c.Statement.LocalCacheSize, c.Statement.LocalCacheTTL)
	}
	statementRepo := repository.NewProblemStatementRepositoryWithTTL(conn, cacheClient, statementLocal, c.Statement.RedisTTL, c.Statement.EmptyTTL)
	uploadRepo := repository.NewProblemUploadRepository(conn)
	metaPublisher := metainvalidation.NewPublisher(metainvalidationClient(c))

	var cleanupConsumer *cleanup.ProblemCleanupConsumer
	var cleanupQueue queue.MessageQueue
	var cleanupPublisher *cleanup.ProblemCleanupPublisher
	var deadLetterPusher *kq.Pusher

	if len(c.Kafka.Brokers) > 0 && c.Cleanup.Topic != "" {
		cleanupConsumer = cleanup.NewProblemCleanupConsumer(
			problemRepo,
			storageClient,
			cleanup.CleanupOptions{
				Bucket:          c.MinIO.Bucket,
				KeyPrefix:       c.Upload.KeyPrefix,
				BatchSize:       c.Cleanup.BatchSize,
				ListTimeout:     c.Cleanup.ListTimeout,
				DeleteTimeout:   c.Cleanup.DeleteTimeout,
				MaxUploads:      c.Cleanup.MaxUploads,
				MaxRetries:      c.Cleanup.MaxRetries,
				RetryDelay:      c.Cleanup.RetryDelay,
				MessageTTL:      c.Cleanup.MessageTTL,
				DeadLetterTopic: c.Cleanup.DeadLetterTopic,
			},
		)

		kqConf := cleanup.BuildCleanupKqConf(c)
		cleanupQueue = kq.MustNewQueue(kqConf, cleanupConsumer)
		cleanupPublisher = cleanup.NewProblemCleanupPublisher(c.Kafka.Brokers, c.Cleanup.Topic, c.MinIO.Bucket, c.Upload.KeyPrefix)
		if c.Cleanup.DeadLetterTopic != "" {
			deadLetterPusher = kq.NewPusher(c.Kafka.Brokers, c.Cleanup.DeadLetterTopic, kq.WithSyncPush())
			cleanupConsumer.SetDeadLetterPusher(deadLetterPusher)
		}
	}

	return &ServiceContext{
		Config:           c,
		Conn:             conn,
		Cache:            cacheClient,
		ProblemRepo:      problemRepo,
		StatementRepo:    statementRepo,
		UploadRepo:       uploadRepo,
		Storage:          storageClient,
		CleanupQueue:     cleanupQueue,
		CleanupConsumer:  cleanupConsumer,
		CleanupPublisher: cleanupPublisher,
		MetaPublisher:    metaPublisher,
		DeadLetterPusher: deadLetterPusher,
	}
}

func metainvalidationClient(c config.Config) *red.Client {
	return metapubsub.NewClient(c.Redis)
}
