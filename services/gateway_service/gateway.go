package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fuzoj/pkg/bootstrap"
	pkgerrors "fuzoj/pkg/errors"
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

var configFile = flag.String("f", "etc/gateway.yaml", "the config file")

func main() {
	flag.Parse()

	var bootCfg struct {
		Bootstrap bootstrap.Config `json:"bootstrap"`
	}
	conf.MustLoad(*configFile, &bootCfg)

	boot := bootCfg.Bootstrap
	if boot.Keys.Config == "" {
		fmt.Fprintln(os.Stderr, "bootstrap.keys.config is required")
		return
	}

	var full config.Config
	if err := bootstrap.LoadConfig(context.Background(), boot.Etcd, boot.Keys.Config, &full); err != nil {
		fmt.Fprintf(os.Stderr, "load full config failed: %v\n", err)
		return
	}
	full.Bootstrap = boot
	cfg := full

	runtime, err := bootstrap.LoadRestRuntime(context.Background(), cfg.Bootstrap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load runtime config failed: %v\n", err)
		return
	}
	changed, err := bootstrap.AssignRandomRestPort(&runtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "assign random rest port failed: %v\n", err)
		return
	}
	if changed {
		if err := bootstrap.PutJSON(context.Background(), cfg.Bootstrap.Etcd, cfg.Bootstrap.Keys.Runtime, runtime); err != nil {
			fmt.Fprintf(os.Stderr, "update runtime config failed: %v\n", err)
			return
		}
	}
	if err := bootstrap.ApplyRestRuntime(&cfg.RestConf, runtime); err != nil {
		fmt.Fprintf(os.Stderr, "apply runtime config failed: %v\n", err)
		return
	}

	var logCfg logx.LogConf
	if err := bootstrap.LoadJSON(context.Background(), cfg.Bootstrap.Etcd, cfg.Bootstrap.Keys.Log, &logCfg); err != nil {
		fmt.Fprintf(os.Stderr, "load log config failed: %v\n", err)
		return
	}
	logx.MustSetup(logCfg)

	if err := cfg.Normalize(); err != nil {
		logx.WithContext(context.Background()).Errorf("load config failed: %v", err)
		return
	}

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
	server.Use(middleware.RateLimitMiddleware(ctx.RateService, cfg.Rate.Window, cfg.Rate.GlobalRefillPerSec, cfg.Rate.GlobalCapacity))
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

	logx.WithContext(context.Background()).Infof("gateway http server started addr=%s", cfg.Host+":"+intToString(cfg.Port))
	registerKey, err := bootstrap.RestRegisterKey(runtime)
	if err != nil {
		logx.WithContext(context.Background()).Errorf("build register key failed: %v", err)
		return
	}
	registerValue, err := bootstrap.RestRegisterValue(runtime)
	if err != nil {
		logx.WithContext(context.Background()).Errorf("build register value failed: %v", err)
		return
	}
	pub, err := bootstrap.RegisterService(cfg.Bootstrap.Etcd, registerKey, registerValue)
	if err != nil {
		logx.WithContext(context.Background()).Errorf("register service failed: %v", err)
		return
	}
	defer pub.Stop()
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
