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
	Bootstrap       bootstrap.Config      `json:"bootstrap,optional"`
	Mysql           MysqlConfig           `json:"mysql"`
	Cache           cache.CacheConf       `json:"cache"`
	Redis           redis.RedisConf       `json:"redis"`
	Kafka           KafkaConfig           `json:"kafka"`
	Topics          TopicConfig           `json:"topics"`
	Contest         ContestConfig         `json:"contest"`
	ContestDispatch ContestDispatchConfig `json:"contestDispatch"`
	Leaderboard     LeaderboardConfig     `json:"leaderboard"`
	Timeouts        TimeoutConfig         `json:"timeouts"`
}

type MysqlConfig struct {
	DataSource string `json:"dataSource"`
}

type KafkaConfig struct {
	Brokers  []string `json:"brokers"`
	ClientID string   `json:"clientID"`
	MinBytes int      `json:"minBytes"`
	MaxBytes int      `json:"maxBytes"`
	Topics   []string `json:"topics"`
}

type TopicConfig struct {
	Level0 string `json:"level0"`
	Level1 string `json:"level1"`
	Level2 string `json:"level2"`
	Level3 string `json:"level3"`
}

type ContestConfig struct {
	IdempotencyTTL            time.Duration `json:"idempotencyTTL"`
	ResultPersistAfter        time.Duration `json:"resultPersistAfter"`
	MaxParticipantsPerTeam    int           `json:"maxParticipantsPerTeam"`
	DefaultPageSize           int           `json:"defaultPageSize"`
	MaxPageSize               int           `json:"maxPageSize"`
	EligibilityCacheTTL       time.Duration `json:"eligibilityCacheTTL"`
	EligibilityEmptyTTL       time.Duration `json:"eligibilityEmptyTTL"`
	EligibilityLocalCacheSize int           `json:"eligibilityLocalCacheSize"`
	EligibilityLocalCacheTTL  time.Duration `json:"eligibilityLocalCacheTTL"`
}

type ContestDispatchConfig struct {
	Topic           string        `json:"topic"`
	ConsumerGroup   string        `json:"consumerGroup"`
	PrefetchCount   int           `json:"prefetchCount"`
	Concurrency     int           `json:"concurrency"`
	MaxRetries      int           `json:"maxRetries"`
	RetryDelay      time.Duration `json:"retryDelay"`
	DeadLetterTopic string        `json:"deadLetterTopic"`
	MessageTTL      time.Duration `json:"messageTTL"`
	IdempotencyTTL  time.Duration `json:"idempotencyTTL"`
	StatusTTL       time.Duration `json:"statusTTL"`
}

type LeaderboardConfig struct {
	HotCacheTTL      time.Duration `json:"hotCacheTTL"`
	PageCacheTTL     time.Duration `json:"pageCacheTTL"`
	EmptyTTL         time.Duration `json:"emptyTTL"`
	FrozenCacheTTL   time.Duration `json:"frozenCacheTTL"`
	SnapshotInterval time.Duration `json:"snapshotInterval"`
}

type TimeoutConfig struct {
	DB    time.Duration `json:"db"`
	Cache time.Duration `json:"cache"`
	MQ    time.Duration `json:"mq"`
}
