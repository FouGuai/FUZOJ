package config

import (
	"time"

	"fuzoj/pkg/bootstrap"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Bootstrap   bootstrap.Config  `json:"bootstrap,optional"`
	Mysql       MysqlConfig       `json:"mysql"`
	Cache       cache.CacheConf   `json:"cache"`
	Kafka       KafkaConfig       `json:"kafka"`
	Contest     ContestConfig     `json:"contest"`
	Leaderboard LeaderboardConfig `json:"leaderboard"`
	Timeouts    TimeoutConfig     `json:"timeouts"`
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

type ContestConfig struct {
	IdempotencyTTL         time.Duration `json:"idempotencyTTL"`
	ResultPersistAfter     time.Duration `json:"resultPersistAfter"`
	MaxParticipantsPerTeam int           `json:"maxParticipantsPerTeam"`
	DefaultPageSize        int           `json:"defaultPageSize"`
	MaxPageSize            int           `json:"maxPageSize"`
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
