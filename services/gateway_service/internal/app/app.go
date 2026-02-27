package app

import (
	"context"
	"fmt"
	pkgerrors "fuzoj/pkg/errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fuzoj/pkg/utils/contextkey"
	"fuzoj/services/gateway_service/internal/config"
	"fuzoj/services/gateway_service/internal/discovery"
	"fuzoj/services/gateway_service/internal/middleware"
	"fuzoj/services/gateway_service/internal/proxy"
	"fuzoj/services/gateway_service/internal/response"
	"fuzoj/services/gateway_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func Run(configPath string) {
	var cfg config.Config
	conf.MustLoad(configPath, &cfg)
	if err := cfg.Normalize(); err != nil {
		logx.WithContext(context.Background()).Errorf("load config failed: %v", err)
		return
	}

	logx.MustSetup(cfg.Logger)

	applyHTTPTransport(cfg.Proxy)
	setErrorHandler()

	ctx, err := svc.NewServiceContext(cfg)
	if err != nil {
		logx.WithContext(context.Background()).Errorf("init service context failed: %v", err)
		return
	}
	defer ctx.Close()

	if cfg.BanEvent.Enabled && ctx.MQClient != nil {
		logx.WithContext(context.Background()).Info("start ban event consumer")
		go ctx.MQClient.Start()
	}

	routes, matcher, err := buildGatewayRoutes(cfg, ctx.Registry)
	if err != nil {
		logx.WithContext(context.Background()).Errorf("build gateway config failed: %v", err)
		return
	}

	server := rest.MustNewServer(cfg.RestConf)
	defer server.Stop()
	server.Use(middleware.TraceMiddleware())
	server.Use(middleware.CORSMiddleware(buildCORSConfig(cfg.CORS)))
	server.Use(middleware.RoutePolicyMiddleware(matcher))
	server.Use(middleware.AuthMiddleware(ctx.AuthService))
	server.Use(middleware.RateLimitMiddleware(ctx.RateService, cfg.Rate.Window))
	server.Use(middleware.RouteMiddleware())
	server.Use(middleware.RequestLogger())
	server.AddRoutes(routes)

	server.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/healthz",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})
	server.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/readyz",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})

	logx.WithContext(context.Background()).Infof("gateway http server started addr=%s", cfg.Host+":"+strconv.Itoa(cfg.Port))
	server.Start()
}

func buildGatewayRoutes(cfg config.Config, registry *discovery.RegistryManager) ([]rest.Route, *middleware.PolicyMatcher, error) {
	matcher := middleware.NewPolicyMatcher()
	routes := make([]rest.Route, 0, len(cfg.Upstreams))

	for _, upstream := range cfg.Upstreams {
		if upstream.Http == nil {
			return nil, nil, fmt.Errorf("upstream http config is required")
		}
		if registry == nil {
			return nil, nil, fmt.Errorf("registry manager is required")
		}
		registryKey := upstream.RegistryKey
		if registryKey == "" {
			if upstream.Name == "" {
				return nil, nil, fmt.Errorf("upstream name or registryKey is required")
			}
			registryKey = upstream.Name + ".rest"
		}
		picker, err := registry.GetPicker(registryKey)
		if err != nil {
			return nil, nil, fmt.Errorf("get registry picker failed: %w", err)
		}
		handler := proxy.NewHTTPForwarder(picker, *upstream.Http)

		for _, mapping := range upstream.Mappings {
			method := strings.ToUpper(mapping.Method)
			if method == "" {
				method = http.MethodGet
			}
			routes = append(routes, rest.Route{
				Method:  method,
				Path:    mapping.Path,
				Handler: handler,
			})

			policy := middleware.RoutePolicy{
				Name:        routeName(mapping),
				Path:        mapping.Path,
				Auth:        middleware.AuthPolicy{Mode: mapping.Auth.Mode, Roles: mapping.Auth.Roles},
				RateLimit:   buildRateLimit(cfg.Rate, mapping.RateLimit),
				Timeout:     mapping.Timeout,
				StripPrefix: mapping.StripPrefix,
			}
			matcher.AddExact(method, mapping.Path, policy)

			if strings.HasSuffix(mapping.Path, "/*any") {
				basePath := strings.TrimSuffix(mapping.Path, "/*any")
				if basePath != "" {
					routes = append(routes, rest.Route{
						Method:  method,
						Path:    basePath,
						Handler: handler,
					})
					basePolicy := policy
					basePolicy.Path = basePath
					matcher.AddExact(method, basePath, basePolicy)
					matcher.AddWildcard(method, basePath, policy)
				}
			}
		}
	}

	return routes, matcher, nil
}

func buildRateLimit(defaults config.RateLimitConfig, override config.RouteRateLimit) middleware.RateLimitPolicy {
	return middleware.RateLimitPolicy{
		Window:   override.Window,
		UserMax:  pickLimit(override.UserMax, defaults.UserMax),
		IPMax:    pickLimit(override.IPMax, defaults.IPMax),
		RouteMax: pickLimit(override.RouteMax, defaults.RouteMax),
	}
}

func pickLimit(routeValue, defaultValue int) int {
	if routeValue > 0 {
		return routeValue
	}
	return defaultValue
}

func routeName(mapping config.RouteMapping) string {
	if mapping.Name != "" {
		return mapping.Name
	}
	return mapping.Path
}

func buildCORSConfig(cfg config.CORSConfig) middleware.CORSConfig {
	maxAge := ""
	if cfg.MaxAge > 0 {
		maxAge = strconv.Itoa(int(cfg.MaxAge.Seconds()))
	}
	return middleware.CORSConfig{
		Enabled:          cfg.Enabled,
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		ExposedHeaders:   cfg.ExposedHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           maxAge,
	}
}

func applyHTTPTransport(cfg config.ProxyConfig) {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
	}
	http.DefaultTransport = transport
	http.DefaultClient.Transport = transport
}

func setErrorHandler() {
	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		customErr := pkgerrors.GetError(err)
		if customErr == nil {
			customErr = pkgerrors.New(pkgerrors.ServiceUnavailable)
		}
		logx.WithContext(ctx).Errorf("gateway upstream error: %v", err)
		resp := response.Response{
			Code:    customErr.Code,
			Message: customErr.Error(),
			TraceID: traceIDFromContext(ctx),
		}
		return customErr.Code.HTTPStatus(), resp
	})
}

func traceIDFromContext(ctx context.Context) string {
	if val := ctx.Value(contextkey.TraceID); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
	}
	return ""
}
