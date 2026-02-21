// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	"fuzoj/judge_service/internal/cache"
	"fuzoj/judge_service/internal/config"
	"fuzoj/judge_service/internal/handler"
	"fuzoj/judge_service/internal/logic"
	"fuzoj/judge_service/internal/problemclient"
	"fuzoj/judge_service/internal/repository"
	"fuzoj/judge_service/internal/sandbox"
	sbconfig "fuzoj/judge_service/internal/sandbox/config"
	"fuzoj/judge_service/internal/sandbox/engine"
	"fuzoj/judge_service/internal/sandbox/runner"
	"fuzoj/judge_service/internal/service"
	"fuzoj/judge_service/internal/svc"

	problemv1 "fuzoj/api/gen/problem/v1"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var configFile = flag.String("f", "etc/judge.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	applyDefaults(&c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

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

	mqClient, err := mq.NewKafkaQueue(c.Kafka.ToMQConfig())
	if err != nil {
		logx.Errorf("init kafka failed: %v", err)
		return
	}
	defer func() {
		_ = mqClient.Stop()
		_ = mqClient.Close()
	}()

	statusPublisher := repository.NewMQStatusEventPublisher(mqClient, c.Status.FinalTopic)
	ctx.StatusRepo = repository.NewStatusRepository(
		ctx.StatusCache,
		ctx.SubmissionsModel,
		c.StatusCacheTTL,
		c.StatusCacheEmptyTTL,
		statusPublisher,
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

	grpcCtx, cancel := context.WithTimeout(context.Background(), c.Problem.Timeout)
	defer cancel()
	grpcConn, err := grpc.DialContext(grpcCtx, c.Problem.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logx.Errorf("init problem grpc client failed: %v", err)
		return
	}
	defer func() {
		_ = grpcConn.Close()
	}()

	problemClient := problemclient.NewClient(problemv1.NewProblemServiceClient(grpcConn))
	judgeSvc, err := service.NewService(service.Config{
		Worker:         worker,
		StatusRepo:     ctx.StatusRepo,
		ProblemClient:  problemClient,
		DataCache:      dataCache,
		Storage:        objStorage,
		Queue:          mqClient,
		SourceBucket:   c.Source.Bucket,
		WorkRoot:       c.Judge.WorkRoot,
		WorkerTimeout:  c.Worker.Timeout,
		ProblemTimeout: c.Problem.Timeout,
		StorageTimeout: c.Source.Timeout,
		StatusTimeout:  c.Status.Timeout,
		MetaTTL:        c.Problem.MetaTTL,
		WorkerPoolSize: c.Worker.PoolSize,
		RetryTopic:     c.Kafka.RetryTopic,
		PoolRetryMax:   c.Kafka.PoolRetryMax,
		PoolRetryBase:  c.Kafka.PoolRetryBase,
		PoolRetryMaxD:  c.Kafka.PoolRetryMaxD,
		DeadLetter:     c.Kafka.DeadLetter,
	})
	if err != nil {
		logx.Errorf("init judge service failed: %v", err)
		return
	}
	ctx.JudgeService = judgeSvc

	weights := c.Kafka.TopicWeights
	if len(weights) == 0 {
		weights = defaultTopicWeights(c.Kafka.Topics)
	}
	weightedTopics, err := buildWeightedTopics(c.Kafka.Topics, weights)
	if err != nil {
		logx.Errorf("build weighted topics failed: %v", err)
		return
	}

	limiter := mq.NewTokenLimiter(c.Worker.PoolSize)
	consumer := logic.NewJudgeConsumerLogic(context.Background(), ctx)
	err = mqClient.SubscribeWeighted(context.Background(), weightedTopics, consumer.HandleMessage, &mq.SubscribeOptions{
		ConsumerGroup:   c.Kafka.ConsumerGroup,
		PrefetchCount:   c.Kafka.PrefetchCount,
		Concurrency:     c.Kafka.Concurrency,
		MaxRetries:      c.Kafka.MaxRetries,
		RetryDelay:      c.Kafka.RetryDelay,
		DeadLetterTopic: c.Kafka.DeadLetter,
		MessageTTL:      c.Kafka.MessageTTL,
	}, limiter)
	if err != nil {
		logx.Errorf("subscribe kafka failed: %v", err)
		return
	}
	if err := mqClient.Start(); err != nil {
		logx.Errorf("start kafka consumer failed: %v", err)
		return
	}
	handler.RegisterHandlers(server, ctx)

	logx.Infof("starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}

func validateConfig(ctx *svc.ServiceContext, c *config.Config) error {
	if ctx == nil || c == nil {
		return fmt.Errorf("service context is required")
	}
	if ctx.StatusCache == nil {
		return fmt.Errorf("status cache is not configured")
	}
	if c.Problem.Addr == "" {
		return fmt.Errorf("problem addr is required")
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

func buildWeightedTopics(topics []string, weights map[string]int) ([]mq.WeightedTopic, error) {
	weighted := make([]mq.WeightedTopic, 0, len(topics))
	for _, topic := range topics {
		weight, ok := weights[topic]
		if !ok || weight <= 0 {
			return nil, fmt.Errorf("invalid topic weight for %s", topic)
		}
		weighted = append(weighted, mq.WeightedTopic{Topic: topic, Weight: weight})
	}
	return weighted, nil
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
