// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"time"

	"fuzoj/pkg/bootstrap"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Bootstrap bootstrap.Config `json:"bootstrap,optional"`
	Mysql     struct {
		DataSource string `json:"dataSource"`
	} `json:"mysql"`
	Cache    cache.CacheConf `json:"cache"`
	Redis    redis.RedisConf `json:"redis"`
	Kafka    KafkaConfig     `json:"kafka"`
	Rank     RankConfig      `json:"rank"`
	Timeouts TimeoutConfig   `json:"timeouts"`
}

type KafkaConfig struct {
	Brokers  []string `json:"brokers"`
	ClientID string   `json:"clientID"`
	MinBytes int      `json:"minBytes"`
	MaxBytes int      `json:"maxBytes"`
}

type RankConfig struct {
	UpdateTopic      string        `json:"updateTopic"`
	ConsumerGroup    string        `json:"consumerGroup"`
	PrefetchCount    int           `json:"prefetchCount"`
	Concurrency      int           `json:"concurrency"`
	BatchSize        int           `json:"batchSize"`
	BatchInterval    time.Duration `json:"batchInterval"`
	HotCacheTTL      time.Duration `json:"hotCacheTTL"`
	PageCacheTTL     time.Duration `json:"pageCacheTTL"`
	EmptyTTL         time.Duration `json:"emptyTTL"`
	WSDebounce       time.Duration `json:"wsDebounce"`
	SnapshotInterval time.Duration `json:"snapshotInterval"`
	SnapshotPageSize int           `json:"snapshotPageSize"`
	SnapshotBatch    int           `json:"snapshotBatch"`
	RecoverOnStart   bool          `json:"recoverOnStart"`
}

type TimeoutConfig struct {
	Cache time.Duration `json:"cache"`
	DB    time.Duration `json:"db"`
	MQ    time.Duration `json:"mq"`
}
