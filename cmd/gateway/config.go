package main

import (
	"fmt"
	"os"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/mq"
	"fuzoj/pkg/utils/logger"

	"gopkg.in/yaml.v3"
)

const (
	defaultHTTPAddr        = "0.0.0.0:8080"
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultIdleTimeout     = 60 * time.Second
	defaultShutdownTimeout = 10 * time.Second
	defaultMaxHeaderBytes  = 1 << 20
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr           string        `yaml:"addr"`
	ReadTimeout    time.Duration `yaml:"readTimeout"`
	WriteTimeout   time.Duration `yaml:"writeTimeout"`
	IdleTimeout    time.Duration `yaml:"idleTimeout"`
	MaxHeaderBytes int           `yaml:"maxHeaderBytes"`
}

// AuthConfig holds JWT settings.
type AuthConfig struct {
	JWTSecret string `yaml:"jwtSecret"`
	JWTIssuer string `yaml:"jwtIssuer"`
}

// BanEventConfig holds ban event consumer settings.
type BanEventConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Topic         string `yaml:"topic"`
	ConsumerGroup string `yaml:"consumerGroup"`
}

// CacheConfig holds cache-related settings.
type CacheConfig struct {
	BanLocalTTL            time.Duration `yaml:"banLocalTTL"`
	BanLocalSize           int           `yaml:"banLocalSize"`
	TokenBlacklistCacheTTL time.Duration `yaml:"tokenBlacklistCacheTTL"`
}

// RateLimitConfig holds gateway rate limit defaults.
type RateLimitConfig struct {
	Window   time.Duration `yaml:"window"`
	UserMax  int           `yaml:"userMax"`
	IPMax    int           `yaml:"ipMax"`
	RouteMax int           `yaml:"routeMax"`
}

// ProxyConfig holds reverse proxy transport settings.
type ProxyConfig struct {
	MaxIdleConns          int           `yaml:"maxIdleConns"`
	MaxIdleConnsPerHost   int           `yaml:"maxIdleConnsPerHost"`
	IdleConnTimeout       time.Duration `yaml:"idleConnTimeout"`
	ResponseHeaderTimeout time.Duration `yaml:"responseHeaderTimeout"`
	TLSHandshakeTimeout   time.Duration `yaml:"tlsHandshakeTimeout"`
	DialTimeout           time.Duration `yaml:"dialTimeout"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowedOrigins"`
	AllowedMethods   []string `yaml:"allowedMethods"`
	AllowedHeaders   []string `yaml:"allowedHeaders"`
	ExposedHeaders   []string `yaml:"exposedHeaders"`
	AllowCredentials bool     `yaml:"allowCredentials"`
	MaxAge           time.Duration `yaml:"maxAge"`
}

// UpstreamConfig defines an upstream service.
type UpstreamConfig struct {
	Name    string `yaml:"name"`
	BaseURL string `yaml:"baseURL"`
}

// AuthPolicy defines auth behavior for a route.
type AuthPolicy struct {
	Mode  string   `yaml:"mode"`  // public | protected
	Roles []string `yaml:"roles"` // optional
}

// RouteRateLimit overrides per-route limits.
type RouteRateLimit struct {
	Window   time.Duration `yaml:"window"`
	UserMax  int           `yaml:"userMax"`
	IPMax    int           `yaml:"ipMax"`
	RouteMax int           `yaml:"routeMax"`
}

// RouteConfig defines a gateway route.
type RouteConfig struct {
	Name        string         `yaml:"name"`
	Methods     []string       `yaml:"methods"`
	Path        string         `yaml:"path"`
	Upstream    string         `yaml:"upstream"`
	Auth        AuthPolicy     `yaml:"auth"`
	RateLimit   RouteRateLimit `yaml:"rateLimit"`
	Timeout     time.Duration  `yaml:"timeout"`
	StripPrefix string         `yaml:"stripPrefix"`
}

// AppConfig holds the gateway configuration.
type AppConfig struct {
	Server   ServerConfig      `yaml:"server"`
	Logger   logger.Config     `yaml:"logger"`
	Auth     AuthConfig        `yaml:"auth"`
	Redis    cache.RedisConfig `yaml:"redis"`
	Kafka    mq.KafkaConfig    `yaml:"kafka"`
	BanEvent BanEventConfig    `yaml:"banEvent"`
	Cache    CacheConfig       `yaml:"cache"`
	Rate     RateLimitConfig   `yaml:"rateLimit"`
	Proxy    ProxyConfig       `yaml:"proxy"`
	CORS     CORSConfig        `yaml:"cors"`
	Upstreams []UpstreamConfig `yaml:"upstreams"`
	Routes   []RouteConfig     `yaml:"routes"`
}

func loadYAML(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file failed: %w", err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse config file failed: %w", err)
	}
	return nil
}

func loadAppConfig(path string) (*AppConfig, error) {
	var cfg AppConfig
	if err := loadYAML(path, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = defaultHTTPAddr
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = defaultReadTimeout
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = defaultWriteTimeout
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = defaultIdleTimeout
	}
	if cfg.Server.MaxHeaderBytes == 0 {
		cfg.Server.MaxHeaderBytes = defaultMaxHeaderBytes
	}

	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("auth.jwtSecret is required")
	}

	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}
	applyRedisDefaults(&cfg.Redis)

	if cfg.Cache.BanLocalTTL == 0 {
		cfg.Cache.BanLocalTTL = 30 * time.Minute
	}
	if cfg.Cache.BanLocalSize == 0 {
		cfg.Cache.BanLocalSize = 100000
	}
	if cfg.Cache.TokenBlacklistCacheTTL == 0 {
		cfg.Cache.TokenBlacklistCacheTTL = 2 * time.Minute
	}

	if cfg.Rate.Window == 0 {
		cfg.Rate.Window = time.Minute
	}

	if len(cfg.Upstreams) == 0 {
		return nil, fmt.Errorf("at least one upstream is required")
	}
	if len(cfg.Routes) == 0 {
		return nil, fmt.Errorf("at least one route is required")
	}
	if cfg.BanEvent.Enabled {
		if cfg.BanEvent.Topic == "" {
			return nil, fmt.Errorf("banEvent.topic is required")
		}
		if len(cfg.Kafka.Brokers) == 0 {
			return nil, fmt.Errorf("kafka brokers are required when banEvent is enabled")
		}
	}

	return &cfg, nil
}

func applyRedisDefaults(cfg *cache.RedisConfig) {
	if cfg == nil {
		return
	}
	defaults := cache.DefaultRedisConfig()
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = defaults.MinRetryBackoff
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = defaults.MaxRetryBackoff
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = defaults.DialTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = defaults.ReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaults.WriteTimeout
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = defaults.PoolSize
	}
	if cfg.MinIdleConns == 0 {
		cfg.MinIdleConns = defaults.MinIdleConns
	}
	if cfg.PoolTimeout == 0 {
		cfg.PoolTimeout = defaults.PoolTimeout
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = defaults.ConnMaxIdleTime
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = defaults.ConnMaxLifetime
	}
}
