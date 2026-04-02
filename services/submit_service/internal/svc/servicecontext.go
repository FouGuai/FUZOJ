// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"fuzoj/internal/common/storage"
	"fuzoj/pkg/submit/statuspubsub"
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

const rpcRoundRobinBalancer = "round_robin"

type ServiceContext struct {
	Config                config.Config
	Conn                  sqlx.SqlConn
	Redis                 *redis.Redis
	SubmissionsModel      model.SubmissionsModel
	SubmissionRepo        repository.SubmissionRepository
	DispatchRepo          repository.SubmissionDispatchRepository
	StatusRepo            *repository.StatusRepository
	LogRepo               *repository.SubmissionLogRepository
	Storage               storage.ObjectStorage
	StatusFinalQueue      queue.MessageQueue
	StatusFinalConsumer   *consumer.StatusFinalConsumer
	DispatchRecoveryRelay *consumer.DispatchRecoveryRelay
	TopicPushers          TopicPushers
	ContestDispatchPusher TopicPusher
	ContestDispatchSwitch *ContestDispatchSwitch
	ContestRpc            contestrpc.ContestRpc
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
	dispatchRepo := repository.NewSubmissionDispatchRepository(conn)
	statusRepo := repository.NewStatusRepository(redisClient, submissionsModel, c.Submit.StatusTTL, c.Submit.StatusEmptyTTL)
	statusRepo.SetStatusPubSub(statuspubsub.NewClient(c.Redis))

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

	var contestDispatchPusher TopicPusher
	if len(c.Kafka.Brokers) > 0 && c.Submit.ContestDispatch.Topic != "" {
		contestDispatchPusher = kq.NewPusher(c.Kafka.Brokers, c.Submit.ContestDispatch.Topic, kq.WithSyncPush())
	}

	var statusFinalConsumer *consumer.StatusFinalConsumer
	var statusFinalQueue queue.MessageQueue
	if len(c.Kafka.Brokers) > 0 && c.Submit.StatusFinalTopic != "" {
		handlers := make([]consumer.FinalStatusHandler, 0, 1)
		if dispatchRepo != nil {
			if handler := consumer.NewDispatchDoneHandler(dispatchRepo); handler != nil {
				handlers = append(handlers, handler)
			}
		}
		statusFinalConsumer = consumer.NewStatusFinalConsumer(statusRepo, logRepo, handlers, consumer.TimeoutConfig{
			DB: c.Submit.Timeouts.DB,
		})
		kqConf := consumer.BuildStatusFinalKqConf(c)
		statusFinalQueue = kq.MustNewQueue(kqConf, statusFinalConsumer)
	}

	ctx := &ServiceContext{
		Config:                c,
		Conn:                  conn,
		Redis:                 redisClient,
		SubmissionsModel:      submissionsModel,
		SubmissionRepo:        submissionRepo,
		DispatchRepo:          dispatchRepo,
		StatusRepo:            statusRepo,
		LogRepo:               logRepo,
		Storage:               storageClient,
		StatusFinalQueue:      statusFinalQueue,
		StatusFinalConsumer:   statusFinalConsumer,
		TopicPushers:          pushers,
		ContestDispatchPusher: contestDispatchPusher,
		ContestRpc:            initContestRpc(c),
	}
	if dispatchRepo != nil {
		ctx.DispatchRecoveryRelay = consumer.NewDispatchRecoveryRelay(dispatchRepo, submissionsModel, ctx, redisClient, consumer.DispatchRecoveryOptions{
			Enabled:       c.Submit.DispatchRecovery.Enabled,
			TimeoutAfter:  c.Submit.DispatchRecovery.TimeoutAfter,
			ScanInterval:  c.Submit.DispatchRecovery.ScanInterval,
			ClaimBatch:    c.Submit.DispatchRecovery.ClaimBatch,
			WorkerCount:   c.Submit.DispatchRecovery.WorkerCount,
			LeaseDuration: c.Submit.DispatchRecovery.LeaseDuration,
			RetryBase:     c.Submit.DispatchRecovery.RetryBase,
			RetryMax:      c.Submit.DispatchRecovery.RetryMax,
			DBTimeout:     c.Submit.DispatchRecovery.DBTimeout,
			MQTimeout:     c.Submit.DispatchRecovery.MQTimeout,
		})
	}
	return ctx
}

func initContestRpc(c config.Config) contestrpc.ContestRpc {
	if len(c.ContestRpc.Etcd.Hosts) == 0 || c.ContestRpc.Etcd.Key == "" {
		return nil
	}
	rpcConf := c.ContestRpc
	rpcConf.BalancerName = rpcRoundRobinBalancer
	logx.Infof("init contest rpc client with balancer=%s key=%s", rpcConf.BalancerName, rpcConf.Etcd.Key)
	client := zrpc.MustNewClient(rpcConf)
	return contestrpc.NewContestRpc(client)
}
