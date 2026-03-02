// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"fuzoj/internal/common/storage"
	"fuzoj/services/contest_rpc_service/contestrpc"
	"fuzoj/services/submit_service/internal/config"
	"fuzoj/services/submit_service/internal/consumer"
	"fuzoj/services/submit_service/internal/model"
	"fuzoj/services/submit_service/internal/repository"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config              config.Config
	Conn                sqlx.SqlConn
	Redis               *redis.Redis
	SubmissionsModel    model.SubmissionsModel
	SubmissionRepo      repository.SubmissionRepository
	StatusRepo          *repository.StatusRepository
	LogRepo             *repository.SubmissionLogRepository
	Storage             storage.ObjectStorage
	StatusFinalQueue    queue.MessageQueue
	StatusFinalConsumer *consumer.StatusFinalConsumer
	TopicPushers        TopicPushers
	ContestRpc          contestrpc.ContestRpc
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	redisClient := redis.MustNewRedis(c.Redis)

	var cacheOptions []cache.Option
	if c.Submit.SubmissionCacheTTL > 0 {
		cacheOptions = append(cacheOptions, cache.WithExpiry(c.Submit.SubmissionCacheTTL))
	}
	if c.Submit.SubmissionEmptyTTL > 0 {
		cacheOptions = append(cacheOptions, cache.WithNotFoundExpiry(c.Submit.SubmissionEmptyTTL))
	}

	submissionsModel := model.NewSubmissionsModel(conn, c.Cache, cacheOptions...)
	submissionRepo := repository.NewSubmissionRepository(submissionsModel)
	statusRepo := repository.NewStatusRepository(redisClient, submissionsModel, c.Submit.StatusTTL, c.Submit.StatusEmptyTTL)

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

	logBucket := c.Submit.LogBucket
	if logBucket == "" {
		logBucket = c.MinIO.Bucket
	}
	logRepo := repository.NewSubmissionLogRepository(
		conn,
		redisClient,
		storageClient,
		logBucket,
		c.Submit.LogKeyPrefix,
		c.Submit.LogMaxInlineBytes,
		c.Submit.LogCacheTTL,
	)

	pushers := TopicPushers{}
	if len(c.Kafka.Brokers) > 0 {
		if c.Topics.Level0 != "" {
			pushers.Level0 = kq.NewPusher(c.Kafka.Brokers, c.Topics.Level0, kq.WithSyncPush())
		}
		if c.Topics.Level1 != "" {
			pushers.Level1 = kq.NewPusher(c.Kafka.Brokers, c.Topics.Level1, kq.WithSyncPush())
		}
		if c.Topics.Level2 != "" {
			pushers.Level2 = kq.NewPusher(c.Kafka.Brokers, c.Topics.Level2, kq.WithSyncPush())
		}
		if c.Topics.Level3 != "" {
			pushers.Level3 = kq.NewPusher(c.Kafka.Brokers, c.Topics.Level3, kq.WithSyncPush())
		}
	}

	var statusFinalConsumer *consumer.StatusFinalConsumer
	var statusFinalQueue queue.MessageQueue
	if len(c.Kafka.Brokers) > 0 && c.Submit.StatusFinalTopic != "" {
		statusFinalConsumer = consumer.NewStatusFinalConsumer(statusRepo, logRepo, nil, consumer.TimeoutConfig{
			DB: c.Submit.Timeouts.DB,
		})
		kqConf := consumer.BuildStatusFinalKqConf(c)
		statusFinalQueue = kq.MustNewQueue(kqConf, statusFinalConsumer)
	}

	return &ServiceContext{
		Config:              c,
		Conn:                conn,
		Redis:               redisClient,
		SubmissionsModel:    submissionsModel,
		SubmissionRepo:      submissionRepo,
		StatusRepo:          statusRepo,
		LogRepo:             logRepo,
		Storage:             storageClient,
		StatusFinalQueue:    statusFinalQueue,
		StatusFinalConsumer: statusFinalConsumer,
		TopicPushers:        pushers,
		ContestRpc:          initContestRpc(c),
	}
}

func initContestRpc(c config.Config) contestrpc.ContestRpc {
	if len(c.ContestRpc.Etcd.Hosts) == 0 || c.ContestRpc.Etcd.Key == "" {
		return nil
	}
	client := zrpc.MustNewClient(c.ContestRpc)
	return contestrpc.NewContestRpc(client)
}
