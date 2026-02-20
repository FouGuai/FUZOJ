package main

import (
	"fmt"
	"os"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	"fuzoj/pkg/utils/logger"

	"gopkg.in/yaml.v3"
)

const (
	defaultHTTPAddr        = "0.0.0.0:8081"
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultIdleTimeout     = 60 * time.Second
	defaultShutdownTimeout = 10 * time.Second
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"readTimeout"`
	WriteTimeout time.Duration `yaml:"writeTimeout"`
	IdleTimeout  time.Duration `yaml:"idleTimeout"`
}

// AuthConfig holds auth service settings.
type AuthConfig struct {
	JWTSecret       string        `yaml:"jwtSecret"`
	JWTIssuer       string        `yaml:"jwtIssuer"`
	AccessTokenTTL  time.Duration `yaml:"accessTokenTTL"`
	RefreshTokenTTL time.Duration `yaml:"refreshTokenTTL"`
	LoginFailTTL    time.Duration `yaml:"loginFailTTL"`
	LoginFailLimit  int           `yaml:"loginFailLimit"`
	Root            RootAccount   `yaml:"root"`
}

// RootAccount holds bootstrap root account settings.
type RootAccount struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Email    string `yaml:"email"`
}

// AppConfig holds the user-service configuration.
type AppConfig struct {
	Server   ServerConfig      `yaml:"server"`
	Logger   logger.Config     `yaml:"logger"`
	Auth     AuthConfig        `yaml:"auth"`
	Database db.MySQLConfig    `yaml:"database"`
	Redis    cache.RedisConfig `yaml:"redis"`
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
	if cfg.Database.DSN == "" {
		return nil, fmt.Errorf("database dsn is required")
	}
	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}
	applyRedisDefaults(&cfg.Redis)

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

	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("auth.jwtSecret is required")
	}
	if cfg.Auth.Root.Enabled {
		if cfg.Auth.Root.Username == "" {
			return nil, fmt.Errorf("auth.root.username is required")
		}
		if cfg.Auth.Root.Password == "" {
			return nil, fmt.Errorf("auth.root.password is required")
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
