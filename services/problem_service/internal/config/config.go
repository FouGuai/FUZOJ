// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"time"

	"fuzoj/pkg/bootstrap"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Bootstrap bootstrap.Config   `json:"bootstrap"`
	Rpc       zrpc.RpcServerConf `json:"rpc"`
	Mysql     struct {
		DataSource string `json:"dataSource"`
	} `json:"mysql"`
	Cache     cache.CacheConf `json:"cache"`
	Redis     redis.RedisConf `json:"redis"`
	Kafka     KafkaConfig     `json:"kafka"`
	MinIO     MinIOConfig     `json:"minio"`
	Upload    UploadConfig    `json:"upload"`
	Cleanup   CleanupConfig   `json:"cleanup"`
	Statement StatementConfig `json:"statement"`
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

// StatementConfig holds statement settings.
type StatementConfig struct {
	MaxBytes       int           `json:"maxBytes"`
	RedisTTL       time.Duration `json:"redisTTL"`
	EmptyTTL       time.Duration `json:"emptyTTL"`
	LocalCacheSize int           `json:"localCacheSize"`
	LocalCacheTTL  time.Duration `json:"localCacheTTL"`
	Timeout        time.Duration `json:"timeout"`
}
