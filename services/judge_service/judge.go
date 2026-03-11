// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"fuzoj/internal/common/mq/weighted_kq"
	"fuzoj/internal/common/storage"
	"fuzoj/pkg/bootstrap"
	"fuzoj/services/judge_service/internal/cache"
	"fuzoj/services/judge_service/internal/config"
	"fuzoj/services/judge_service/internal/handler"
	"fuzoj/services/judge_service/internal/logic"
	"fuzoj/services/judge_service/internal/problemclient"
	"fuzoj/services/judge_service/internal/repository"
	"fuzoj/services/judge_service/internal/sandbox"
	sbconfig "fuzoj/services/judge_service/internal/sandbox/config"
	"fuzoj/services/judge_service/internal/sandbox/engine"
	"fuzoj/services/judge_service/internal/sandbox/runner"
	"fuzoj/services/judge_service/internal/svc"

	problemv1 "fuzoj/api/gen/problem/v1"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

var configFile = flag.String("f", "etc/judge.yaml", "the config file")

const consumerRetryDelay = 3 * time.Second

func main() {
	flag.Parse()

	var bootCfg struct {
		Bootstrap bootstrap.Config `json:"bootstrap"`
	}
	conf.MustLoad(*configFile, &bootCfg)

	boot := bootCfg.Bootstrap
	if boot.Keys.Config == "" {
		logx.Error("bootstrap.keys.config is required")
		return
	}

	var full config.Config
	if err := bootstrap.LoadConfig(context.Background(), boot.Etcd, boot.Keys.Config, &full); err != nil {
		logx.Errorf("load full config failed: %v", err)
		return
	}
	full.Bootstrap = boot
	c := full
	applyDefaults(&c)

	runtime, err := bootstrap.LoadRestRuntime(context.Background(), c.Bootstrap)
	if err != nil {
		logx.Errorf("load runtime config failed: %v", err)
		return
	}
	changed, err := bootstrap.AssignRandomRestPort(&runtime)
	if err != nil {
		logx.Errorf("assign random rest port failed: %v", err)
		return
	}
	if changed {
		if err := bootstrap.PutJSON(context.Background(), c.Bootstrap.Etcd, c.Bootstrap.Keys.Runtime, runtime); err != nil {
			logx.Errorf("update runtime config failed: %v", err)
			return
		}
	}
	if err := bootstrap.ApplyRestRuntime(&c.RestConf, runtime); err != nil {
		logx.Errorf("apply runtime config failed: %v", err)
		return
	}

	var logConf logx.LogConf
	if err := bootstrap.LoadJSON(context.Background(), c.Bootstrap.Etcd, c.Bootstrap.Keys.Log, &logConf); err != nil {
		logx.Errorf("load log config failed: %v", err)
		return
	}
	logx.MustSetup(logConf)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	registerKey, err := bootstrap.RestRegisterKey(runtime)
	if err != nil {
		logx.Errorf("build register key failed: %v", err)
		return
	}
	registerValue, err := bootstrap.RestRegisterValue(runtime)
	if err != nil {
		logx.Errorf("build register value failed: %v", err)
		return
	}
	pub, err := bootstrap.RegisterService(c.Bootstrap.Etcd, registerKey, registerValue)
	if err != nil {
		logx.Errorf("register service failed: %v", err)
		return
	}
	defer pub.Stop()

	ctx := svc.NewServiceContext(c)
	if err := validateConfig(ctx, &c); err != nil {
		logx.Errorf("invalid config: %v", err)
		return
	}
	objStorage, err := storage.NewMinIOStorage(toMinIOConfig(c.MinIO))
	if err != nil {
		logx.Errorf("init minio failed: %v", err)
		return
	}
	localRepo := sbconfig.NewLocalRepository(c.Language.Languages, c.Language.Profiles)
	eng, err := engine.NewEngine(c.Sandbox.ToEngineConfig(), localRepo)
	if err != nil {
		logx.Errorf("init sandbox engine failed: %v", err)
		return
	}
	jobRunner := runner.NewRunner(eng)
	worker := sandbox.NewWorker(jobRunner, localRepo, localRepo)
	ctx.Worker = worker

	if len(c.Kafka.Brokers) == 0 {
		logx.Error("kafka brokers are required")
		return
	}
	var statusPusher *kq.Pusher
	if c.Status.FinalTopic != "" {
		statusPusher = kq.NewPusher(c.Kafka.Brokers, c.Status.FinalTopic, kq.WithSyncPush())
	}
	var retryPusher *kq.Pusher
	if c.Kafka.RetryTopic != "" {
		retryPusher = kq.NewPusher(c.Kafka.Brokers, c.Kafka.RetryTopic, kq.WithSyncPush())
	}
	var deadLetterPusher *kq.Pusher
	if c.Kafka.DeadLetter != "" {
		deadLetterPusher = kq.NewPusher(c.Kafka.Brokers, c.Kafka.DeadLetter, kq.WithSyncPush())
	}
	defer func() {
		if statusPusher != nil {
			_ = statusPusher.Close()
		}
		if retryPusher != nil {
			_ = retryPusher.Close()
		}
		if deadLetterPusher != nil {
			_ = deadLetterPusher.Close()
		}
	}()

	ctx.StatusPusher = statusPusher
	ctx.RetryPusher = retryPusher
	ctx.DeadLetterPusher = deadLetterPusher

	statusPublisher := repository.NewMQStatusEventPublisher(statusPusher, c.Status.FinalTopic)
	finalBatcher := repository.NewFinalStatusBatcher(
		ctx.SubmissionsModel,
		statusPublisher,
		c.Status.FinalBatchSize,
		c.Status.FinalBatchInterval,
		c.Status.FinalBatchTimeout,
	)
	finalBatcher.Start()
	defer finalBatcher.Stop()
	ctx.FinalStatusBatcher = finalBatcher
	ctx.StatusRepo = repository.NewStatusRepository(
		ctx.StatusCache,
		ctx.SubmissionsModel,
		c.StatusCacheTTL,
		c.StatusCacheEmptyTTL,
		finalBatcher,
	)

	dataCache := cache.NewDataPackCache(
		c.CacheConfig.RootDir,
		c.CacheConfig.TTL,
		c.CacheConfig.LockWait,
		c.CacheConfig.MaxEntries,
		c.CacheConfig.MaxBytes,
		c.MinIO.Bucket,
		objStorage,
		ctx.StatusCache,
	)
	ctx.DataCache = dataCache
	ctx.Storage = objStorage

	rpcClient, err := zrpc.NewClient(c.ProblemRpc.RpcClientConf)
	if err != nil {
		logx.Errorf("init problem rpc client failed: %v", err)
		return
	}

	problemClient := problemclient.NewClient(problemv1.NewProblemServiceClient(rpcClient.Conn()))
	ctx.ProblemClient = problemClient

	go startConsumerLoop(ctx, &c)
	handler.RegisterHandlers(server, ctx)

	logx.Infof("starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}

func startConsumerLoop(ctx *svc.ServiceContext, c *config.Config) {
	for {
		consumer := logic.NewJudgeConsumerLogic(context.Background(), ctx)
		confs, err := weighted_kq.BuildWeightedKqConfs(weighted_kq.WeightedKqOptions{
			Brokers:          c.Kafka.Brokers,
			Group:            c.Kafka.ConsumerGroup,
			Topics:           c.Kafka.Topics,
			TopicWeights:     c.Kafka.TopicWeights,
			ConsumersTotal:   c.Kafka.PrefetchCount,
			ProcessorsTotal:  c.Kafka.Concurrency,
			MinBytes:         c.Kafka.MinBytes,
			MaxBytes:         c.Kafka.MaxBytes,
			ServiceName:      c.Name,
			RetryTopic:       c.Kafka.RetryTopic,
			RetryMaxInFlight: c.Kafka.RetryMaxInFlight,
			AutoAddRetry:     true,
		})
		if err != nil {
			logx.Errorf("build weighted kq configs failed: %v", err)
			time.Sleep(consumerRetryDelay)
			continue
		}
		logx.Infof("judge consumer config topics=%v group=%s brokers=%v", c.Kafka.Topics, c.Kafka.ConsumerGroup, c.Kafka.Brokers)
		queueGroup, err := weighted_kq.NewWeightedKqQueuesWithPolicy(confs, consumer, weighted_kq.WeightedQueuePolicy{
			TopicWeights:     c.Kafka.TopicWeights,
			RetryTopic:       c.Kafka.RetryTopic,
			RetryMaxInFlight: c.Kafka.RetryMaxInFlight,
		})
		if err != nil {
			logx.Errorf("init kq consumers failed: %v", err)
			time.Sleep(consumerRetryDelay)
			continue
		}
		logx.Infof("judge consumer group started")
		queueGroup.Start()
		queueGroup.Stop()
		logx.Infof("judge consumer group stopped, restarting in %s", consumerRetryDelay)
		time.Sleep(consumerRetryDelay)
	}
}

func validateConfig(ctx *svc.ServiceContext, c *config.Config) error {
	if ctx == nil || c == nil {
		return fmt.Errorf("service context is required")
	}
	if ctx.StatusCache == nil {
		return fmt.Errorf("status cache is not configured")
	}
	if len(c.ProblemRpc.Etcd.Hosts) == 0 || c.ProblemRpc.Etcd.Key == "" {
		return fmt.Errorf("problem rpc etcd config is required")
	}
	if c.ProblemRpc.CallTimeout <= 0 {
		return fmt.Errorf("problem rpc call timeout is required")
	}
	if len(c.Kafka.Topics) == 0 {
		return fmt.Errorf("kafka topics are required")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka brokers are required")
	}
	return nil
}

func applyDefaults(c *config.Config) {
	if c == nil {
		return
	}
	if c.Source.Bucket == "" {
		c.Source.Bucket = c.MinIO.Bucket
	}
	if c.Worker.PoolSize <= 0 {
		c.Worker.PoolSize = 1
	}
	if c.Status.FinalTopic == "" {
		c.Status.FinalTopic = "judge.status.final"
	}
	if c.Status.FinalBatchSize <= 0 {
		c.Status.FinalBatchSize = 100
	}
	if c.Status.FinalBatchInterval <= 0 {
		c.Status.FinalBatchInterval = 100 * time.Millisecond
	}
	if c.Status.FinalBatchTimeout <= 0 {
		c.Status.FinalBatchTimeout = 3 * time.Second
	}
	if c.Kafka.RetryTopic == "" {
		c.Kafka.RetryTopic = "judge.retry"
	}
	if c.Kafka.PoolRetryMax <= 0 {
		c.Kafka.PoolRetryMax = 5
	}
	if c.Kafka.PoolRetryBase == 0 {
		c.Kafka.PoolRetryBase = time.Second
	}
	if c.Kafka.PoolRetryMaxD == 0 {
		c.Kafka.PoolRetryMaxD = 30 * time.Second
	}
	if c.ProblemRpc.CallTimeout == 0 {
		c.ProblemRpc.CallTimeout = 3 * time.Second
	}
	if len(c.Kafka.TopicWeights) == 0 && len(c.Kafka.Topics) > 0 {
		c.Kafka.TopicWeights = defaultTopicWeights(c.Kafka.Topics)
	}
}

func defaultTopicWeights(topics []string) map[string]int {
	weights := []int{8, 4, 2, 1}
	out := make(map[string]int, len(topics))
	for i, topic := range topics {
		if topic == "" {
			continue
		}
		if i < len(weights) {
			out[topic] = weights[i]
			continue
		}
		out[topic] = 1
	}
	return out
}

func toMinIOConfig(cfg config.MinIOConfig) storage.MinIOConfig {
	return storage.MinIOConfig{
		Endpoint:   cfg.Endpoint,
		AccessKey:  cfg.AccessKey,
		SecretKey:  cfg.SecretKey,
		UseSSL:     cfg.UseSSL,
		Bucket:     cfg.Bucket,
		PresignTTL: cfg.PresignTTL,
	}
}
