package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	commonmw "fuzoj/internal/common/http/middleware"
	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	judgecache "fuzoj/judge_service/internal/cache"
	"fuzoj/judge_service/internal/controller"
	"fuzoj/judge_service/internal/problemclient"
	"fuzoj/judge_service/internal/repository"
	"fuzoj/judge_service/internal/sandbox"
	"fuzoj/judge_service/internal/sandbox/config"
	"fuzoj/judge_service/internal/sandbox/engine"
	"fuzoj/judge_service/internal/sandbox/runner"
	"fuzoj/judge_service/internal/service"
	"fuzoj/pkg/utils/logger"

	problemv1 "fuzoj/api/gen/problem/v1"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultConfigPath = "configs/judge_service.yaml"

func main() {
	configPath := flag.String("config", defaultConfigPath, "Path to config file")
	flag.Parse()

	appCfg, err := loadAppConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load app config failed: %v\n", err)
		return
	}

	if err := logger.Init(appCfg.Logger); err != nil {
		fmt.Fprintf(os.Stderr, "init logger failed: %v\n", err)
		return
	}
	defer func() {
		_ = logger.Sync()
	}()

	mysqlDB, err := db.NewMySQLWithConfig(&appCfg.Database)
	if err != nil {
		logger.Error(context.Background(), "init database failed", zap.Error(err))
		return
	}
	defer func() {
		_ = mysqlDB.Close()
	}()
	dbProvider := db.NewManager(mysqlDB)

	redisCache, err := cache.NewRedisCacheWithConfig(&appCfg.Redis)
	if err != nil {
		logger.Error(context.Background(), "init redis failed", zap.Error(err))
		return
	}
	defer func() {
		_ = redisCache.Close()
	}()

	objStorage, err := storage.NewMinIOStorage(appCfg.MinIO)
	if err != nil {
		logger.Error(context.Background(), "init minio failed", zap.Error(err))
		return
	}

	mqClient, err := mq.NewKafkaQueue(appCfg.Kafka.toMQConfig())
	if err != nil {
		logger.Error(context.Background(), "init kafka failed", zap.Error(err))
		return
	}
	defer func() {
		_ = mqClient.Close()
	}()

	statusPublisher := repository.NewMQStatusEventPublisher(mqClient, appCfg.Status.FinalTopic)
	localRepo := config.NewLocalRepository(appCfg.Language.Languages, appCfg.Language.Profiles)
	eng, err := engine.NewEngine(appCfg.Sandbox.toEngineConfig(), localRepo)
	if err != nil {
		logger.Error(context.Background(), "init sandbox engine failed", zap.Error(err))
		return
	}
	jobRunner := runner.NewRunner(eng)
	worker := sandbox.NewWorker(jobRunner, localRepo, localRepo)

	statusRepo := repository.NewStatusRepository(redisCache, dbProvider, appCfg.Status.TTL, statusPublisher)
	dataCache := judgecache.NewDataPackCache(appCfg.Cache.RootDir, appCfg.Cache.TTL, appCfg.Cache.LockWait, appCfg.Cache.MaxEntries, appCfg.Cache.MaxBytes, appCfg.MinIO.Bucket, objStorage, redisCache)

	grpcConn, err := grpc.Dial(appCfg.Problem.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error(context.Background(), "init problem grpc client failed", zap.Error(err))
		return
	}
	defer func() {
		_ = grpcConn.Close()
	}()

	problemClient := problemclient.NewClient(problemv1.NewProblemServiceClient(grpcConn))
	judgeSvc, err := service.NewService(service.Config{
		Worker:         worker,
		StatusRepo:     statusRepo,
		ProblemClient:  problemClient,
		DataCache:      dataCache,
		Storage:        objStorage,
		Queue:          mqClient,
		SourceBucket:   appCfg.Source.Bucket,
		WorkRoot:       appCfg.Judge.WorkRoot,
		WorkerTimeout:  appCfg.Worker.Timeout,
		ProblemTimeout: appCfg.Problem.Timeout,
		StorageTimeout: appCfg.Source.Timeout,
		StatusTimeout:  appCfg.Status.Timeout,
		MetaTTL:        appCfg.Problem.MetaTTL,
		WorkerPoolSize: appCfg.Worker.PoolSize,
		RetryTopic:     appCfg.Kafka.RetryTopic,
		PoolRetryMax:   appCfg.Kafka.PoolRetryMax,
		PoolRetryBase:  appCfg.Kafka.PoolRetryBase,
		PoolRetryMaxD:  appCfg.Kafka.PoolRetryMaxD,
		DeadLetter:     appCfg.Kafka.DeadLetter,
	})
	if err != nil {
		logger.Error(context.Background(), "init judge service failed", zap.Error(err))
		return
	}

	if len(appCfg.Kafka.Topics) == 0 {
		logger.Error(context.Background(), "kafka topics are required")
		return
	}
	weights := appCfg.Kafka.TopicWeights
	if len(weights) == 0 {
		weights = defaultTopicWeights(appCfg.Kafka.Topics)
	}
	weightedTopics := make([]mq.WeightedTopic, 0, len(appCfg.Kafka.Topics))
	for _, topic := range appCfg.Kafka.Topics {
		weight, ok := weights[topic]
		if !ok || weight <= 0 {
			logger.Error(context.Background(), "invalid topic weight", zap.String("topic", topic), zap.Int("weight", weight))
			return
		}
		weightedTopics = append(weightedTopics, mq.WeightedTopic{Topic: topic, Weight: weight})
	}

	limiter := mq.NewTokenLimiter(appCfg.Worker.PoolSize)
	err = mqClient.SubscribeWeighted(context.Background(), weightedTopics, judgeSvc.HandleMessage, &mq.SubscribeOptions{
		ConsumerGroup:   appCfg.Kafka.ConsumerGroup,
		PrefetchCount:   appCfg.Kafka.PrefetchCount,
		Concurrency:     appCfg.Kafka.Concurrency,
		MaxRetries:      appCfg.Kafka.MaxRetries,
		RetryDelay:      appCfg.Kafka.RetryDelay,
		DeadLetterTopic: appCfg.Kafka.DeadLetter,
		MessageTTL:      appCfg.Kafka.MessageTTL,
	}, limiter)
	if err != nil {
		logger.Error(context.Background(), "subscribe kafka failed", zap.Error(err))
		return
	}
	if err := mqClient.Start(); err != nil {
		logger.Error(context.Background(), "start kafka consumer failed", zap.Error(err))
		return
	}

	httpServer := buildHTTPServer(appCfg.Server, statusRepo)
	listener, err := net.Listen("tcp", appCfg.Server.Addr)
	if err != nil {
		logger.Error(context.Background(), "init http listener failed", zap.Error(err))
		return
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info(context.Background(), "judge http server started", zap.String("addr", appCfg.Server.Addr))
		errCh <- httpServer.Serve(listener)
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(context.Background(), "http server stopped", zap.Error(err))
		}
	case <-shutdownCtx.Done():
		logger.Info(context.Background(), "shutdown signal received")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error(context.Background(), "http server shutdown failed", zap.Error(err))
	}
	_ = mqClient.Stop()
}

func buildHTTPServer(cfg ServerConfig, statusRepo *repository.StatusRepository) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(commonmw.TraceContextMiddleware())
	router.Use(requestLogger())

	api := router.Group("/api/v1/judge")
	judgeController := controller.NewJudgeController(statusRepo)
	api.GET("/submissions/:id", judgeController.GetStatus)

	return &http.Server{
		Addr:         cfg.Addr,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		logger.Info(
			c.Request.Context(),
			"request completed",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
