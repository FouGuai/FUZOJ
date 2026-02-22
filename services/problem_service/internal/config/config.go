// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Grpc struct {
		Addr string `json:"addr"`
	} `json:"grpc"`
	Mysql struct {
		DataSource string `json:"dataSource"`
	} `json:"mysql"`
	Cache   cache.CacheConf `json:"cache"`
	Redis   redis.RedisConf `json:"redis"`
	Kafka   KafkaConfig     `json:"kafka"`
	MinIO   MinIOConfig     `json:"minio"`
	Upload  UploadConfig    `json:"upload"`
	Cleanup CleanupConfig   `json:"cleanup"`
}

// KafkaConfig holds Kafka settings for kq.
type KafkaConfig struct {
	Brokers  []string `json:"brokers"`
	ClientID string   `json:"clientID"`
	MinBytes int      `json:"minBytes"`
	MaxBytes int      `json:"maxBytes"`
}

// MinIOConfig holds object storage settings.
type MinIOConfig struct {
	Endpoint   string        `json:"endpoint"`
	AccessKey  string        `json:"accessKey"`
	SecretKey  string        `json:"secretKey"`
	UseSSL     bool          `json:"useSSL"`
	Bucket     string        `json:"bucket"`
	PresignTTL time.Duration `json:"presignTTL"`
}

// UploadConfig holds upload session settings.
type UploadConfig struct {
	KeyPrefix     string        `json:"keyPrefix"`
	PartSizeBytes int64         `json:"partSizeBytes"`
	SessionTTL    time.Duration `json:"sessionTTL"`
	PresignTTL    time.Duration `json:"presignTTL"`
}

// CleanupConfig holds cleanup consumer settings.
type CleanupConfig struct {
	Topic           string        `json:"topic"`
	ConsumerGroup   string        `json:"consumerGroup"`
	PrefetchCount   int           `json:"prefetchCount"`
	Concurrency     int           `json:"concurrency"`
	MaxRetries      int           `json:"maxRetries"`
	RetryDelay      time.Duration `json:"retryDelay"`
	DeadLetterTopic string        `json:"deadLetterTopic"`
	MessageTTL      time.Duration `json:"messageTTL"`

	BatchSize     int           `json:"batchSize"`
	ListTimeout   time.Duration `json:"listTimeout"`
	DeleteTimeout time.Duration `json:"deleteTimeout"`
	MaxUploads    int           `json:"maxUploads"`
}
