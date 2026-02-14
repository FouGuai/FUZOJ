package main

import (
	"fmt"
	"os"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	"fuzoj/internal/submit/service"
	"fuzoj/pkg/utils/logger"

	"gopkg.in/yaml.v3"
)

const (
	defaultHTTPAddr        = "0.0.0.0:8086"
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

// TopicConfig defines judge topic routing.
type TopicConfig struct {
	Level0 string `yaml:"level0"`
	Level1 string `yaml:"level1"`
	Level2 string `yaml:"level2"`
	Level3 string `yaml:"level3"`
}

// SubmitConfig holds submission settings.
type SubmitConfig struct {
	SourceBucket       string                  `yaml:"sourceBucket"`
	SourceKeyPrefix    string                  `yaml:"sourceKeyPrefix"`
	MaxCodeBytes       int                     `yaml:"maxCodeBytes"`
	IdempotencyTTL     time.Duration           `yaml:"idempotencyTTL"`
	BatchLimit         int                     `yaml:"batchLimit"`
	StatusTTL          time.Duration           `yaml:"statusTTL"`
	SubmissionCacheTTL time.Duration           `yaml:"submissionCacheTTL"`
	SubmissionEmptyTTL time.Duration           `yaml:"submissionEmptyTTL"`
	RateLimit          service.RateLimitConfig `yaml:"rateLimit"`
	Timeouts           service.TimeoutConfig   `yaml:"timeouts"`
}

// AppConfig holds submit-service configuration.
type AppConfig struct {
	Server ServerConfig        `yaml:"server"`
	Logger logger.Config       `yaml:"logger"`
	Kafka  mq.KafkaConfig      `yaml:"kafka"`
	Topics TopicConfig         `yaml:"topics"`
	MinIO  storage.MinIOConfig `yaml:"minio"`
	Submit SubmitConfig        `yaml:"submit"`
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

	if cfg.Topics.Level0 == "" {
		cfg.Topics.Level0 = "judge.level0"
	}
	if cfg.Topics.Level1 == "" {
		cfg.Topics.Level1 = "judge.level1"
	}
	if cfg.Topics.Level2 == "" {
		cfg.Topics.Level2 = "judge.level2"
	}
	if cfg.Topics.Level3 == "" {
		cfg.Topics.Level3 = "judge.level3"
	}

	if cfg.Submit.MaxCodeBytes == 0 {
		cfg.Submit.MaxCodeBytes = 256 * 1024
	}
	if cfg.Submit.IdempotencyTTL == 0 {
		cfg.Submit.IdempotencyTTL = 10 * time.Minute
	}
	if cfg.Submit.BatchLimit == 0 {
		cfg.Submit.BatchLimit = 200
	}
	if cfg.Submit.StatusTTL == 0 {
		cfg.Submit.StatusTTL = 24 * time.Hour
	}
	if cfg.Submit.SubmissionCacheTTL == 0 {
		cfg.Submit.SubmissionCacheTTL = 30 * time.Minute
	}
	if cfg.Submit.SubmissionEmptyTTL == 0 {
		cfg.Submit.SubmissionEmptyTTL = 5 * time.Minute
	}
	if cfg.Submit.RateLimit.Window == 0 {
		cfg.Submit.RateLimit.Window = time.Minute
	}
	if cfg.Submit.RateLimit.UserMax == 0 {
		cfg.Submit.RateLimit.UserMax = 30
	}
	if cfg.Submit.RateLimit.IPMax == 0 {
		cfg.Submit.RateLimit.IPMax = 60
	}
	if cfg.Submit.Timeouts.DB == 0 {
		cfg.Submit.Timeouts.DB = 3 * time.Second
	}
	if cfg.Submit.Timeouts.Cache == 0 {
		cfg.Submit.Timeouts.Cache = 1 * time.Second
	}
	if cfg.Submit.Timeouts.MQ == 0 {
		cfg.Submit.Timeouts.MQ = 3 * time.Second
	}
	if cfg.Submit.Timeouts.Storage == 0 {
		cfg.Submit.Timeouts.Storage = 5 * time.Second
	}
	if cfg.Submit.Timeouts.Status == 0 {
		cfg.Submit.Timeouts.Status = 2 * time.Second
	}

	if cfg.Submit.SourceBucket == "" {
		cfg.Submit.SourceBucket = cfg.MinIO.Bucket
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
