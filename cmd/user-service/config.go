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
}

// AppConfig holds the user-service configuration.
type AppConfig struct {
	Server ServerConfig  `yaml:"server"`
	Logger logger.Config `yaml:"logger"`
	Auth   AuthConfig    `yaml:"auth"`
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

	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("auth.jwtSecret is required")
	}

	return &cfg, nil
}

func loadDatabaseConfig(path string) (*db.MySQLConfig, error) {
	var cfg db.MySQLConfig
	if err := loadYAML(path, &cfg); err != nil {
		return nil, err
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database dsn is required")
	}
	return &cfg, nil
}

func loadRedisConfig(path string) (*cache.RedisConfig, error) {
	var cfg cache.RedisConfig
	if err := loadYAML(path, &cfg); err != nil {
		return nil, err
	}
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}
	return &cfg, nil
}
