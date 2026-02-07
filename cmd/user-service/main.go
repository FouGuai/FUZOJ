package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fuzoj/internal/user"

	"go.uber.org/zap"
)

const (
	defaultDatabaseConfigPath = "configs/database.yaml"
	defaultRedisConfigPath    = "configs/redis.yaml"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		os.Exit(1)
	}
	defer func() {
		_ = logger.Sync()
	}()

	databasePath := getenvWithDefault("USER_SERVICE_DATABASE_CONFIG", defaultDatabaseConfigPath)
	redisPath := getenvWithDefault("USER_SERVICE_REDIS_CONFIG", defaultRedisConfigPath)
	configSource := user.NewFileConfigSource(databasePath, redisPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	deps, err := user.InitUserService(initCtx, configSource)
	if err != nil {
		logger.Error("init user service failed", zap.Error(err))
		os.Exit(1)
	}
	defer func() {
		if closeErr := deps.Close(); closeErr != nil {
			logger.Error("close dependencies failed", zap.Error(closeErr))
		}
	}()

	logger.Info("user service initialized", zap.String("databaseConfig", databasePath), zap.String("redisConfig", redisPath))

	<-ctx.Done()
	logger.Info("user service shutting down", zap.String("signal", ctx.Err().Error()))
}

func getenvWithDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
