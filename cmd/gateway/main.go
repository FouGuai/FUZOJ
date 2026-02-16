package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/mq"
	"fuzoj/internal/gateway/middleware"
	"fuzoj/internal/gateway/repository"
	"fuzoj/internal/gateway/service"
	"fuzoj/pkg/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const defaultConfigPath = "configs/gateway.yaml"

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
	defer func() { _ = logger.Sync() }()

	redisCache, err := cache.NewRedisCacheWithConfig(&appCfg.Redis)
	if err != nil {
		logger.Error(context.Background(), "init redis failed", zap.Error(err))
		return
	}
	defer func() { _ = redisCache.Close() }()

	var mqClient mq.MessageQueue
	if appCfg.BanEvent.Enabled {
		mqClient, err = mq.NewKafkaQueue(appCfg.Kafka)
		if err != nil {
			logger.Error(context.Background(), "init kafka failed", zap.Error(err))
			return
		}
		defer func() { _ = mqClient.Close() }()
	}

	banLocal := repository.NewLRUCache(appCfg.Cache.BanLocalSize, appCfg.Cache.BanLocalTTL)
	banRepo := repository.NewBanCacheRepository(banLocal, redisCache, appCfg.Redis.ReadTimeout, appCfg.Cache.BanLocalTTL)
	tokenLocal := repository.NewLRUCache(tokenCacheSize(appCfg.Cache.BanLocalSize), appCfg.Cache.TokenBlacklistCacheTTL)
	blacklistRepo := repository.NewTokenBlacklistRepository(tokenLocal, redisCache, appCfg.Redis.ReadTimeout, appCfg.Cache.TokenBlacklistCacheTTL)

	authService := service.NewAuthService(appCfg.Auth.JWTSecret, appCfg.Auth.JWTIssuer, blacklistRepo, banRepo)
	rateService := service.NewRateLimitService(redisCache, appCfg.Rate.Window, appCfg.Redis.ReadTimeout)

	upstreams, err := parseUpstreams(appCfg.Upstreams)
	if err != nil {
		logger.Error(context.Background(), "parse upstreams failed", zap.Error(err))
		return
	}

	proxyFactory := service.NewProxyFactory(service.ProxyConfig{
		MaxIdleConns:          appCfg.Proxy.MaxIdleConns,
		MaxIdleConnsPerHost:   appCfg.Proxy.MaxIdleConnsPerHost,
		IdleConnTimeout:       appCfg.Proxy.IdleConnTimeout,
		ResponseHeaderTimeout: appCfg.Proxy.ResponseHeaderTimeout,
		TLSHandshakeTimeout:   appCfg.Proxy.TLSHandshakeTimeout,
		DialTimeout:           appCfg.Proxy.DialTimeout,
	}, upstreams)

	if appCfg.BanEvent.Enabled {
		consumer := service.NewBanEventConsumer(mqClient, banRepo, appCfg.Cache.BanLocalTTL)
		if err := consumer.Subscribe(context.Background(), appCfg.BanEvent.Topic, appCfg.BanEvent.ConsumerGroup); err != nil {
			logger.Error(context.Background(), "subscribe ban events failed", zap.Error(err))
			return
		}
	}

	httpServer, err := buildHTTPServer(appCfg, authService, rateService, proxyFactory)
	if err != nil {
		logger.Error(context.Background(), "build http server failed", zap.Error(err))
		return
	}

	listener, err := net.Listen("tcp", appCfg.Server.Addr)
	if err != nil {
		logger.Error(context.Background(), "init http listener failed", zap.Error(err))
		return
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info(context.Background(), "gateway http server started", zap.String("addr", appCfg.Server.Addr))
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
}

func buildHTTPServer(cfg *AppConfig, authService *service.AuthService, rateService *service.RateLimitService, proxyFactory *service.ProxyFactory) (*http.Server, error) {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.TraceMiddleware())
	maxAge := ""
	if cfg.CORS.MaxAge > 0 {
		maxAge = fmt.Sprintf("%d", int(cfg.CORS.MaxAge.Seconds()))
	}
	router.Use(middleware.CORSMiddleware(middleware.CORSConfig{
		Enabled:          cfg.CORS.Enabled,
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   cfg.CORS.ExposedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           maxAge,
	}))
	router.Use(requestLogger())

	router.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/readyz", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, route := range cfg.Routes {
		proxy, err := proxyFactory.Get(route.Upstream)
		if err != nil {
			return nil, fmt.Errorf("resolve upstream %s failed: %w", route.Upstream, err)
		}
		routeKey := route.Name
		if routeKey == "" {
			routeKey = route.Path
		}
		policy := middleware.AuthPolicy{Mode: route.Auth.Mode, Roles: route.Auth.Roles}
		ratePolicy := middleware.RateLimitPolicy{
			Window:   route.RateLimit.Window,
			UserMax:  pickLimit(route.RateLimit.UserMax, cfg.Rate.UserMax),
			IPMax:    pickLimit(route.RateLimit.IPMax, cfg.Rate.IPMax),
			RouteMax: pickLimit(route.RateLimit.RouteMax, cfg.Rate.RouteMax),
		}

		handlers := []gin.HandlerFunc{
			middleware.AuthMiddleware(authService, policy),
			middleware.RateLimitMiddleware(rateService, routeKey, ratePolicy, cfg.Rate.Window),
			middleware.ProxyHandler(proxy, routeKey, route.Timeout, route.StripPrefix),
		}
		if len(route.Methods) == 0 {
			route.Methods = []string{http.MethodGet}
		}
		router.Handle(route.Methods, route.Path, handlers...)
	}

	return &http.Server{
		Addr:           cfg.Server.Addr,
		Handler:        router,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}, nil
}

func pickLimit(routeValue, defaultValue int) int {
	if routeValue > 0 {
		return routeValue
	}
	return defaultValue
}

func tokenCacheSize(banCacheSize int) int {
	size := banCacheSize / 10
	if size < 1024 {
		size = 1024
	}
	return size
}

func parseUpstreams(items []UpstreamConfig) (map[string]*url.URL, error) {
	result := make(map[string]*url.URL, len(items))
	for _, item := range items {
		if item.Name == "" || item.BaseURL == "" {
			return nil, fmt.Errorf("upstream name and baseURL are required")
		}
		parsed, err := url.Parse(item.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("parse upstream %s failed: %w", item.Name, err)
		}
		result[item.Name] = parsed
	}
	return result, nil
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
