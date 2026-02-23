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
	Mysql struct {
		DataSource string `json:"dataSource"`
	} `json:"mysql"`
	Cache  cache.CacheConf `json:"cache"`
	Redis  redis.RedisConf `json:"redis"`
	Kafka  KafkaConfig     `json:"kafka"`
	MinIO  MinIOConfig     `json:"minio"`
	Topics TopicConfig     `json:"topics"`
	Submit SubmitConfig    `json:"submit"`
}

type KafkaConfig struct {
	Brokers  []string `json:"brokers"`
	ClientID string   `json:"clientID"`
	MinBytes int      `json:"minBytes"`
	MaxBytes int      `json:"maxBytes"`
}

type MinIOConfig struct {
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	UseSSL    bool   `json:"useSSL"`
	Bucket    string `json:"bucket"`
}

type TopicConfig struct {
	Level0 string `json:"level0"`
	Level1 string `json:"level1"`
	Level2 string `json:"level2"`
	Level3 string `json:"level3"`
}

type SubmitConfig struct {
	SourceBucket        string          `json:"sourceBucket"`
	SourceKeyPrefix     string          `json:"sourceKeyPrefix"`
	MaxCodeBytes        int             `json:"maxCodeBytes"`
	IdempotencyTTL      time.Duration   `json:"idempotencyTTL"`
	BatchLimit          int             `json:"batchLimit"`
	StatusTTL           time.Duration   `json:"statusTTL"`
	StatusEmptyTTL      time.Duration   `json:"statusEmptyTTL"`
	StatusFinalTopic    string          `json:"statusFinalTopic"`
	StatusFinalConsumer ConsumerConfig  `json:"statusFinalConsumer"`
	SubmissionCacheTTL  time.Duration   `json:"submissionCacheTTL"`
	SubmissionEmptyTTL  time.Duration   `json:"submissionEmptyTTL"`
	RateLimit           RateLimitConfig `json:"rateLimit"`
	Timeouts            TimeoutConfig   `json:"timeouts"`
}

type ConsumerConfig struct {
	ConsumerGroup   string        `json:"consumerGroup"`
	PrefetchCount   int           `json:"prefetchCount"`
	Concurrency     int           `json:"concurrency"`
	MaxRetries      int           `json:"maxRetries"`
	RetryDelay      time.Duration `json:"retryDelay"`
	DeadLetterTopic string        `json:"deadLetterTopic"`
	MessageTTL      time.Duration `json:"messageTTL"`
}

type RateLimitConfig struct {
	UserMax int           `json:"userMax"`
	IPMax   int           `json:"ipMax"`
	Window  time.Duration `json:"window"`
}

type TimeoutConfig struct {
	DB      time.Duration `json:"db"`
	Cache   time.Duration `json:"cache"`
	MQ      time.Duration `json:"mq"`
	Storage time.Duration `json:"storage"`
	Status  time.Duration `json:"status"`
}
