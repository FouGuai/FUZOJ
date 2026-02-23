package config

import (
	"fmt"
	"time"

	"fuzoj/pkg/utils/logger"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

const (
	defaultBanLocalTTL            = 30 * time.Minute
	defaultBanLocalSize           = 100000
	defaultTokenBlacklistCacheTTL = 2 * time.Minute
	defaultRateWindow             = time.Minute
)

// AuthConfig holds JWT settings.
type AuthConfig struct {
	JWTSecret string `json:"jwtSecret"`
	JWTIssuer string `json:"jwtIssuer"`
}

// BanEventConfig holds ban event consumer settings.
type BanEventConfig struct {
	Enabled       bool   `json:"enabled"`
	Topic         string `json:"topic"`
	ConsumerGroup string `json:"consumerGroup"`
}

// CacheConfig holds cache-related settings.
type CacheConfig struct {
	BanLocalTTL            time.Duration `json:"banLocalTTL"`
	BanLocalSize           int           `json:"banLocalSize"`
	TokenBlacklistCacheTTL time.Duration `json:"tokenBlacklistCacheTTL"`
}

// RateLimitConfig holds gateway rate limit defaults.
type RateLimitConfig struct {
	Window   time.Duration `json:"window"`
	UserMax  int           `json:"userMax"`
	IPMax    int           `json:"ipMax"`
	RouteMax int           `json:"routeMax"`
}

// ProxyConfig holds reverse proxy transport settings.
type ProxyConfig struct {
	MaxIdleConns          int           `json:"maxIdleConns"`
	MaxIdleConnsPerHost   int           `json:"maxIdleConnsPerHost"`
	IdleConnTimeout       time.Duration `json:"idleConnTimeout"`
	ResponseHeaderTimeout time.Duration `json:"responseHeaderTimeout"`
	TLSHandshakeTimeout   time.Duration `json:"tlsHandshakeTimeout"`
	DialTimeout           time.Duration `json:"dialTimeout"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	Enabled          bool          `json:"enabled"`
	AllowedOrigins   []string      `json:"allowedOrigins"`
	AllowedMethods   []string      `json:"allowedMethods"`
	AllowedHeaders   []string      `json:"allowedHeaders"`
	ExposedHeaders   []string      `json:"exposedHeaders"`
	AllowCredentials bool          `json:"allowCredentials"`
	MaxAge           time.Duration `json:"maxAge"`
}

// AuthPolicy defines auth behavior for a route.
type AuthPolicy struct {
	Mode  string   `json:"mode"`  // public | protected
	Roles []string `json:"roles"` // optional
}

// RouteRateLimit overrides per-route limits.
type RouteRateLimit struct {
	Window   time.Duration `json:"window"`
	UserMax  int           `json:"userMax"`
	IPMax    int           `json:"ipMax"`
	RouteMax int           `json:"routeMax"`
}

// RouteMapping defines a gateway route mapping and policy.
type RouteMapping struct {
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	RpcPath     string         `json:"rpcPath,optional"`
	Name        string         `json:"name,optional"`
	Auth        AuthPolicy     `json:"auth"`
	RateLimit   RouteRateLimit `json:"rateLimit"`
	Timeout     time.Duration  `json:"timeout"`
	StripPrefix string         `json:"stripPrefix,optional"`
}

// HttpClientConf is the configuration for an HTTP client.
type HttpClientConf struct {
	Target  string `json:"target"`
	Prefix  string `json:"prefix,optional"`
	Timeout int64  `json:"timeout,default=3000"`
}

// Upstream is the configuration for an upstream.
type Upstream struct {
	Name      string         `json:"name,optional"`
	Http      *HttpClientConf `json:"http,optional"`
	ProtoSets []string       `json:"protoSets,optional"`
	Mappings  []RouteMapping `json:"mappings,optional"`
}

// Config holds the gateway configuration.
type Config struct {
	rest.RestConf
	Upstreams []Upstream       `json:"upstreams"`
	Auth      AuthConfig       `json:"auth"`
	Redis     redis.RedisConf  `json:"redis"`
	Kafka     KafkaConfig      `json:"kafka"`
	BanEvent  BanEventConfig   `json:"banEvent"`
	Cache     CacheConfig      `json:"cache"`
	Rate      RateLimitConfig  `json:"rateLimit"`
	Proxy     ProxyConfig      `json:"proxy"`
	CORS      CORSConfig       `json:"cors"`
	Logger    logger.Config    `json:"logger"`
}

// KafkaConfig holds Kafka client settings for kq.
type KafkaConfig struct {
	Brokers       []string `json:"brokers"`
	Username      string   `json:"username,optional"`
	Password      string   `json:"password,optional"`
	CaFile        string   `json:"caFile,optional"`
	Conns         int      `json:"conns,default=1"`
	Consumers     int      `json:"consumers,default=8"`
	Processors    int      `json:"processors,default=8"`
	MinBytes      int      `json:"minBytes,default=10240"`
	MaxBytes      int      `json:"maxBytes,default=10485760"`
	ForceCommit   bool     `json:"forceCommit,default=true"`
	CommitInOrder bool     `json:"commitInOrder,default=false"`
}

// Normalize validates config and applies defaults.
func (c *Config) Normalize() error {
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwtSecret is required")
	}
	if c.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}

	if c.Cache.BanLocalTTL == 0 {
		c.Cache.BanLocalTTL = defaultBanLocalTTL
	}
	if c.Cache.BanLocalSize == 0 {
		c.Cache.BanLocalSize = defaultBanLocalSize
	}
	if c.Cache.TokenBlacklistCacheTTL == 0 {
		c.Cache.TokenBlacklistCacheTTL = defaultTokenBlacklistCacheTTL
	}
	if c.Rate.Window == 0 {
		c.Rate.Window = defaultRateWindow
	}

	if len(c.Upstreams) == 0 {
		return fmt.Errorf("at least one upstream is required")
	}

	if c.BanEvent.Enabled {
		if c.BanEvent.Topic == "" {
			return fmt.Errorf("banEvent.topic is required")
		}
		if len(c.Kafka.Brokers) == 0 {
			return fmt.Errorf("kafka brokers are required when banEvent is enabled")
		}
	}

	return nil
}

// BuildKqConf builds go-queue kq config from gateway config.
func (c *Config) BuildKqConf() kq.KqConf {
	conf := kq.KqConf{
		Brokers:       c.Kafka.Brokers,
		Group:         c.BanEvent.ConsumerGroup,
		Topic:         c.BanEvent.Topic,
		Conns:         c.Kafka.Conns,
		Consumers:     c.Kafka.Consumers,
		Processors:    c.Kafka.Processors,
		MinBytes:      c.Kafka.MinBytes,
		MaxBytes:      c.Kafka.MaxBytes,
		Username:      c.Kafka.Username,
		Password:      c.Kafka.Password,
		CaFile:        c.Kafka.CaFile,
		ForceCommit:   c.Kafka.ForceCommit,
		CommitInOrder: c.Kafka.CommitInOrder,
	}
	return conf
}
