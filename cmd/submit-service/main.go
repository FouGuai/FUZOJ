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
	"fuzoj/internal/judge/repository"
	"fuzoj/internal/submit/controller"
	submitRepo "fuzoj/internal/submit/repository"
	"fuzoj/internal/submit/service"
	"fuzoj/pkg/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const defaultConfigPath = "configs/submit_service.yaml"

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

	mqClient, err := mq.NewKafkaQueue(appCfg.Kafka)
	if err != nil {
		logger.Error(context.Background(), "init kafka failed", zap.Error(err))
		return
	}
	defer func() {
		_ = mqClient.Close()
	}()

	objStorage, err := storage.NewMinIOStorage(appCfg.MinIO)
	if err != nil {
		logger.Error(context.Background(), "init minio failed", zap.Error(err))
		return
	}

	statusRepo := repository.NewStatusRepository(redisCache, dbProvider, appCfg.Submit.StatusTTL, nil)
	submissionRepo := submitRepo.NewSubmissionRepositoryWithTTL(dbProvider, redisCache, appCfg.Submit.SubmissionCacheTTL, appCfg.Submit.SubmissionEmptyTTL)

	submitService, err := service.NewSubmitService(service.Config{
		SubmissionRepo: submissionRepo,
		StatusRepo:     statusRepo,
		Storage:        objStorage,
		MQ:             mqClient,
		Cache:          redisCache,
		Topics: service.TopicConfig{
			Level0: appCfg.Topics.Level0,
			Level1: appCfg.Topics.Level1,
			Level2: appCfg.Topics.Level2,
			Level3: appCfg.Topics.Level3,
		},
		SourceBucket:    appCfg.Submit.SourceBucket,
		SourceKeyPrefix: appCfg.Submit.SourceKeyPrefix,
		MaxCodeBytes:    appCfg.Submit.MaxCodeBytes,
		IdempotencyTTL:  appCfg.Submit.IdempotencyTTL,
		BatchLimit:      appCfg.Submit.BatchLimit,
		RateLimit:       appCfg.Submit.RateLimit,
		Timeouts:        appCfg.Submit.Timeouts,
	})
	if err != nil {
		logger.Error(context.Background(), "init submit service failed", zap.Error(err))
		return
	}

	statusConsumerOpts := appCfg.Submit.StatusFinalConsumer.toSubscribeOptions()
	statusConsumerOpts.SetDefaults()
	if err := mqClient.SubscribeWithOptions(context.Background(), appCfg.Submit.StatusFinalTopic, submitService.HandleFinalStatusMessage, &statusConsumerOpts); err != nil {
		logger.Error(context.Background(), "subscribe status final topic failed", zap.Error(err))
		return
	}
	if err := mqClient.Start(); err != nil {
		logger.Error(context.Background(), "start kafka consumer failed", zap.Error(err))
		return
	}

	httpServer := buildHTTPServer(appCfg.Server, submitService)
	listener, err := net.Listen("tcp", appCfg.Server.Addr)
	if err != nil {
		logger.Error(context.Background(), "init http listener failed", zap.Error(err))
		return
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info(context.Background(), "submit http server started", zap.String("addr", appCfg.Server.Addr))
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

func buildHTTPServer(cfg ServerConfig, submitService *service.SubmitService) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(commonmw.TraceContextMiddleware())
	router.Use(requestLogger())

	api := router.Group("/api/v1/submissions")
	submitController := controller.NewSubmitController(submitService)
	api.POST("", submitController.Create)
	api.GET("/:id", submitController.GetStatus)
	api.POST("/batch_status", submitController.BatchStatus)
	api.GET("/:id/source", submitController.GetSource)

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
