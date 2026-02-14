package main

import (
	"fmt"
	"os"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	"fuzoj/internal/common/storage"
	"fuzoj/pkg/utils/logger"

	"gopkg.in/yaml.v3"
)

const (
	defaultHTTPAddr        = "0.0.0.0:8083"
	defaultGRPCAddr        = "0.0.0.0:9093"
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

// GRPCConfig holds gRPC server settings.
type GRPCConfig struct {
	Addr string `yaml:"addr"`
}

// AppConfig holds the problem-service configuration.
type AppConfig struct {
	Server ServerConfig  `yaml:"server"`
	GRPC   GRPCConfig    `yaml:"grpc"`
	Logger logger.Config `yaml:"logger"`

	MinIO  storage.MinIOConfig `yaml:"minio"`
	Upload UploadConfig        `yaml:"upload"`
}

// UploadConfig holds problem upload settings.
type UploadConfig struct {
	// PartSizeBytes is the multipart upload part size in bytes.
	PartSizeBytes int64 `yaml:"partSizeBytes"`
	// SessionTTL defines how long an upload session is valid.
	SessionTTL time.Duration `yaml:"sessionTTL"`
	// PresignTTL defines how long presigned URLs are valid.
	PresignTTL time.Duration `yaml:"presignTTL"`
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
	if cfg.GRPC.Addr == "" {
		cfg.GRPC.Addr = defaultGRPCAddr
	}

	// Upload defaults.
	if cfg.Upload.PartSizeBytes <= 0 {
		cfg.Upload.PartSizeBytes = 16 * 1024 * 1024
	}
	if cfg.Upload.SessionTTL == 0 {
		cfg.Upload.SessionTTL = 2 * time.Hour
	}
	if cfg.Upload.PresignTTL == 0 {
		cfg.Upload.PresignTTL = 15 * time.Minute
	}

	// Keep MinIOConfig.PresignTTL in sync if not set explicitly.
	if cfg.MinIO.PresignTTL == 0 {
		cfg.MinIO.PresignTTL = cfg.Upload.PresignTTL
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
