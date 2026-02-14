package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	"fuzoj/internal/common/storage"
	"fuzoj/internal/problem/controller"
	"fuzoj/internal/problem/repository"
	"fuzoj/internal/problem/rpc"
	"fuzoj/internal/problem/service"
	"fuzoj/pkg/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	databaseConfigPath = "configs/database.yaml"
	redisConfigPath    = "configs/redis.yaml"
	appConfigPath      = "configs/problem_service.yaml"
)

func main() {
	appCfg, err := loadAppConfig(appConfigPath)
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

	dbCfg, err := loadDatabaseConfig(databaseConfigPath)
	if err != nil {
		logger.Error(context.Background(), "load database config failed", zap.Error(err))
		return
	}

	redisCfg, err := loadRedisConfig(redisConfigPath)
	if err != nil {
		logger.Error(context.Background(), "load redis config failed", zap.Error(err))
		return
	}

	mysqlDB, err := db.NewMySQLWithConfig(dbCfg)
	if err != nil {
		logger.Error(context.Background(), "init database failed", zap.Error(err))
		return
	}
	defer func() {
		_ = mysqlDB.Close()
	}()

	redisCache, err := cache.NewRedisCacheWithConfig(redisCfg)
	if err != nil {
		logger.Error(context.Background(), "init redis failed", zap.Error(err))
		return
	}
	defer func() {
		_ = redisCache.Close()
	}()

	problemRepo := repository.NewProblemRepository(mysqlDB, redisCache)
	problemService := service.NewProblemService(problemRepo)

	objStorage, err := storage.NewMinIOStorage(appCfg.MinIO)
	if err != nil {
		logger.Error(context.Background(), "init minio failed", zap.Error(err))
		return
	}

	uploadRepo := repository.NewProblemUploadRepository(mysqlDB)
	uploadService := service.NewProblemUploadServiceWithDB(mysqlDB, problemRepo, uploadRepo, objStorage, service.UploadOptions{
		Bucket:        appCfg.MinIO.Bucket,
		KeyPrefix:     "problems",
		PartSizeBytes: appCfg.Upload.PartSizeBytes,
		SessionTTL:    appCfg.Upload.SessionTTL,
		PresignTTL:    appCfg.Upload.PresignTTL,
	})

	httpServer := buildHTTPServer(appCfg.Server, problemService, uploadService)
	grpcServer := grpc.NewServer()
	rpc.RegisterProblemService(grpcServer, problemService)

	grpcListener, err := net.Listen("tcp", appCfg.GRPC.Addr)
	if err != nil {
		logger.Error(context.Background(), "init grpc listener failed", zap.Error(err))
		return
	}

	errCh := make(chan error, 2)
	go func() {
		logger.Info(context.Background(), "problem http server started", zap.String("addr", appCfg.Server.Addr))
		errCh <- httpServer.ListenAndServe()
	}()
	go func() {
		logger.Info(context.Background(), "problem grpc server started", zap.String("addr", appCfg.GRPC.Addr))
		errCh <- grpcServer.Serve(grpcListener)
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(context.Background(), "server stopped", zap.Error(err))
		}
	case <-shutdownCtx.Done():
		logger.Info(context.Background(), "shutdown signal received")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error(context.Background(), "http server shutdown failed", zap.Error(err))
	}
	grpcServer.GracefulStop()
}

func buildHTTPServer(cfg ServerConfig, problemService *service.ProblemService, uploadService *service.ProblemUploadService) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger())

	api := router.Group("/api/v1/problems")
	problemController := controller.NewProblemController(problemService)
	api.POST("", problemController.Create)
	api.GET("/:id/latest", problemController.GetLatest)
	api.DELETE("/:id", problemController.Delete)

	uploadController := controller.NewProblemUploadController(uploadService)
	api.POST("/:id/data-pack/uploads:prepare", uploadController.Prepare)
	api.POST("/:id/data-pack/uploads/:upload_id:sign", uploadController.Sign)
	api.POST("/:id/data-pack/uploads/:upload_id:complete", uploadController.Complete)
	api.POST("/:id/data-pack/uploads/:upload_id:abort", uploadController.Abort)
	api.POST("/:id/versions/:version:publish", uploadController.Publish)

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
