package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/pkg/utils/logger"
	"fuzoj/services/gateway_service/internal/config"
	"fuzoj/services/gateway_service/internal/middleware"
	"fuzoj/services/gateway_service/internal/response"
	"fuzoj/services/gateway_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/gateway"
	"github.com/zeromicro/go-zero/rest/httpx"
	"go.uber.org/zap"
)

var configFile = flag.String("f", "etc/gateway.yaml", "the config file")

func main() {
	flag.Parse()

	var cfg config.Config
	conf.MustLoad(*configFile, &cfg)
	if err := cfg.Normalize(); err != nil {
		logger.Error(context.Background(), "load config failed", zap.Error(err))
		return
	}

	if err := logger.Init(cfg.Logger); err != nil {
		logger.Error(context.Background(), "init logger failed", zap.Error(err))
		return
	}
	defer func() { _ = logger.Sync() }()

	applyHTTPTransport(cfg.Proxy)
	setErrorHandler()

	ctx, err := svc.NewServiceContext(cfg)
	if err != nil {
		logger.Error(context.Background(), "init service context failed", zap.Error(err))
		return
	}
	defer ctx.Close()

	if cfg.BanEvent.Enabled && ctx.MQClient != nil {
		logger.Info(context.Background(), "start ban event consumer")
		go ctx.MQClient.Start()
	}

	gwConf, matcher, err := buildGatewayConf(cfg)
	if err != nil {
		logger.Error(context.Background(), "build gateway config failed", zap.Error(err))
		return
	}

	server := gateway.MustNewServer(gwConf, gateway.WithMiddleware(
		middleware.TraceMiddleware(),
		middleware.CORSMiddleware(buildCORSConfig(cfg.CORS)),
		middleware.RoutePolicyMiddleware(matcher),
		middleware.AuthMiddleware(ctx.AuthService),
		middleware.RateLimitMiddleware(ctx.RateService, cfg.Rate.Window),
		middleware.RouteMiddleware(),
		middleware.RequestLogger(),
	))
	defer server.Stop()

	server.AddRoute(gateway.Route{
		Method: http.MethodGet,
		Path:   "/healthz",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})
	server.AddRoute(gateway.Route{
		Method: http.MethodGet,
		Path:   "/readyz",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})

	logger.Info(context.Background(), "gateway http server started", zap.String("addr", cfg.Host+":"+intToString(cfg.Port)))
	server.Start()
}

func buildGatewayConf(cfg config.Config) (gateway.GatewayConf, *middleware.PolicyMatcher, error) {
	matcher := middleware.NewPolicyMatcher()
	upstreams := make([]gateway.Upstream, 0, len(cfg.Upstreams))

	for _, upstream := range cfg.Upstreams {
		if upstream.Http == nil {
			return gateway.GatewayConf{}, nil, errors.New(errors.InvalidParams).WithMessage("upstream http config is required")
		}
		gwUp := gateway.Upstream{
			Name:      upstream.Name,
			Http:      &gateway.HttpClientConf{Target: upstream.Http.Target, Prefix: upstream.Http.Prefix, Timeout: upstream.Http.Timeout},
			ProtoSets: upstream.ProtoSets,
		}

		for _, mapping := range upstream.Mappings {
			method := strings.ToUpper(mapping.Method)
			if method == "" {
				method = http.MethodGet
			}
			gwUp.Mappings = append(gwUp.Mappings, gateway.RouteMapping{
				Method:  method,
				Path:    mapping.Path,
				RpcPath: mapping.RpcPath,
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
					gwUp.Mappings = append(gwUp.Mappings, gateway.RouteMapping{
						Method: method,
						Path:   basePath,
					})
					basePolicy := policy
					basePolicy.Path = basePath
					matcher.AddExact(method, basePath, basePolicy)
					matcher.AddWildcard(method, basePath, policy)
				}
			}
		}
		upstreams = append(upstreams, gwUp)
	}

	return gateway.GatewayConf{RestConf: cfg.RestConf, Upstreams: upstreams}, matcher, nil
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
		maxAge = intToString(int(cfg.MaxAge.Seconds()))
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
		DialContext:           (&netDialer{timeout: cfg.DialTimeout}).DialContext,
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
		customErr := errors.GetError(err)
		if customErr == nil {
			customErr = errors.New(errors.ServiceUnavailable)
		}
		logger.Error(ctx, "gateway upstream error", zap.Error(err))
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

func intToString(value int) string {
	return strconv.Itoa(value)
}

type netDialer struct {
	timeout time.Duration
}

func (d *netDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: d.timeout, KeepAlive: 30 * time.Second}
	return dialer.DialContext(ctx, network, address)
}
