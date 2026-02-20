package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	commonmw "fuzoj/internal/common/http/middleware"
	"fuzoj/internal/user/controller"
	"fuzoj/internal/user/repository"
	"fuzoj/internal/user/service"
	"fuzoj/pkg/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const defaultConfigPath = "configs/user_service.yaml"

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

	userRepo := repository.NewUserRepository(dbProvider, redisCache)
	tokenRepo := repository.NewTokenRepository(dbProvider, redisCache)

	authService := service.NewAuthService(
		dbProvider,
		userRepo,
		tokenRepo,
		redisCache,
		service.AuthServiceConfig{
			JWTSecret:       []byte(appCfg.Auth.JWTSecret),
			JWTIssuer:       appCfg.Auth.JWTIssuer,
			AccessTokenTTL:  appCfg.Auth.AccessTokenTTL,
			RefreshTokenTTL: appCfg.Auth.RefreshTokenTTL,
			LoginFailTTL:    appCfg.Auth.LoginFailTTL,
			LoginFailLimit:  appCfg.Auth.LoginFailLimit,
		},
	)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(commonmw.TraceContextMiddleware())
	router.Use(requestLogger())

	api := router.Group("/api/v1/user")
	authController := controller.NewAuthController(authService)
	api.POST("/register", authController.Register)
	api.POST("/login", authController.Login)
	api.POST("/refresh-token", authController.Refresh)
	api.POST("/logout", authController.Logout)

	srv := &http.Server{
		Addr:         appCfg.Server.Addr,
		Handler:      router,
		ReadTimeout:  appCfg.Server.ReadTimeout,
		WriteTimeout: appCfg.Server.WriteTimeout,
		IdleTimeout:  appCfg.Server.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info(context.Background(), "user service started", zap.String("addr", appCfg.Server.Addr))
		errCh <- srv.ListenAndServe()
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
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error(context.Background(), "http server shutdown failed", zap.Error(err))
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
