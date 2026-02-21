// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"strings"
	"time"

	"fuzoj/internal/common/mq"
	"fuzoj/judge_service/internal/sandbox/engine"
	"fuzoj/judge_service/internal/sandbox/profile"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Mysql struct {
		DataSource string `json:"dataSource"`
	} `json:"mysql"`
	Cache               cache.CacheConf `json:"cache"`
	Redis               redis.RedisConf `json:"redis"`
	StatusCacheTTL      time.Duration   `json:"statusCacheTTL"`
	StatusCacheEmptyTTL time.Duration   `json:"statusCacheEmptyTTL"`
	Kafka               KafkaConfig     `json:"kafka"`
	MinIO               MinIOConfig     `json:"minio"`
	CacheConfig         CacheConfig     `json:"cacheConfig"`
	Worker              WorkerConfig    `json:"worker"`
	Source              SourceConfig    `json:"source"`
	Problem             ProblemConfig   `json:"problem"`
	Status              StatusConfig    `json:"status"`
	Judge               JudgeConfig     `json:"judge"`
	Sandbox             SandboxConfig   `json:"sandbox"`
	Language            LanguageConfig  `json:"language"`
}

// KafkaConfig holds Kafka settings.
type KafkaConfig struct {
	Brokers       []string       `json:"brokers"`
	ClientID      string         `json:"clientID"`
	MinBytes      int            `json:"minBytes"`
	MaxBytes      int            `json:"maxBytes"`
	MaxWait       time.Duration  `json:"maxWait"`
	BatchSize     int            `json:"batchSize"`
	BatchTimeout  time.Duration  `json:"batchTimeout"`
	DialTimeout   time.Duration  `json:"dialTimeout"`
	ReadTimeout   time.Duration  `json:"readTimeout"`
	WriteTimeout  time.Duration  `json:"writeTimeout"`
	RequiredAcks  int            `json:"requiredAcks"`
	Compression   string         `json:"compression"`
	Topics        []string       `json:"topics"`
	ConsumerGroup string         `json:"consumerGroup"`
	PrefetchCount int            `json:"prefetchCount"`
	Concurrency   int            `json:"concurrency"`
	MaxRetries    int            `json:"maxRetries"`
	RetryDelay    time.Duration  `json:"retryDelay"`
	RetryTopic    string         `json:"retryTopic"`
	PoolRetryMax  int            `json:"poolRetryMax"`
	PoolRetryBase time.Duration  `json:"poolRetryBaseDelay"`
	PoolRetryMaxD time.Duration  `json:"poolRetryMaxDelay"`
	DeadLetter    string         `json:"deadLetter"`
	MessageTTL    time.Duration  `json:"messageTTL"`
	TopicWeights  map[string]int `json:"topicWeights"`
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

// CacheConfig holds local data pack cache settings.
type CacheConfig struct {
	RootDir    string        `json:"rootDir"`
	TTL        time.Duration `json:"ttl"`
	LockWait   time.Duration `json:"lockWait"`
	MaxEntries int           `json:"maxEntries"`
	MaxBytes   int64         `json:"maxBytes"`
}

// WorkerConfig holds worker pool settings.
type WorkerConfig struct {
	PoolSize int           `json:"poolSize"`
	Timeout  time.Duration `json:"timeout"`
}

// SourceConfig holds source download settings.
type SourceConfig struct {
	Bucket  string        `json:"bucket"`
	Timeout time.Duration `json:"timeout"`
}

// ProblemConfig holds problem service settings.
type ProblemConfig struct {
	Addr    string        `json:"addr"`
	Timeout time.Duration `json:"timeout"`
	MetaTTL time.Duration `json:"metaTTL"`
}

// StatusConfig holds status persistence settings.
type StatusConfig struct {
	TTL        time.Duration `json:"ttl"`
	Timeout    time.Duration `json:"timeout"`
	FinalTopic string        `json:"finalTopic"`
}

// JudgeConfig holds judge runtime settings.
type JudgeConfig struct {
	WorkRoot string `json:"workRoot"`
}

// SandboxConfig holds sandbox engine settings.
type SandboxConfig struct {
	CgroupRoot           string `json:"cgroupRoot"`
	SeccompDir           string `json:"seccompDir"`
	HelperPath           string `json:"helperPath"`
	StdoutStderrMaxBytes int64  `json:"stdoutStderrMaxBytes"`
	EnableSeccomp        bool   `json:"enableSeccomp"`
	EnableCgroup         bool   `json:"enableCgroup"`
	EnableNamespaces     bool   `json:"enableNamespaces"`
}

// LanguageConfig holds language definitions.
type LanguageConfig struct {
	Languages []profile.LanguageSpec `json:"languages"`
	Profiles  []profile.TaskProfile  `json:"profiles"`
}

// ToMQConfig converts kafka settings to mq.KafkaConfig.
func (k KafkaConfig) ToMQConfig() mq.KafkaConfig {
	cfg := mq.KafkaConfig{
		Brokers:      k.Brokers,
		ClientID:     k.ClientID,
		MinBytes:     k.MinBytes,
		MaxBytes:     k.MaxBytes,
		MaxWait:      k.MaxWait,
		BatchSize:    k.BatchSize,
		BatchTimeout: k.BatchTimeout,
		DialTimeout:  k.DialTimeout,
		ReadTimeout:  k.ReadTimeout,
		WriteTimeout: k.WriteTimeout,
		RequiredAcks: kafka.RequiredAcks(k.RequiredAcks),
	}
	cfg.Compression = parseCompression(k.Compression)
	return cfg
}

// ToEngineConfig converts sandbox settings to engine.Config.
func (s SandboxConfig) ToEngineConfig() engine.Config {
	return engine.Config{
		CgroupRoot:           s.CgroupRoot,
		SeccompDir:           s.SeccompDir,
		HelperPath:           s.HelperPath,
		StdoutStderrMaxBytes: s.StdoutStderrMaxBytes,
		EnableSeccomp:        s.EnableSeccomp,
		EnableCgroup:         s.EnableCgroup,
		EnableNamespaces:     s.EnableNamespaces,
	}
}

func parseCompression(raw string) kafka.Compression {
	switch strings.ToLower(raw) {
	case "gzip":
		return kafka.Gzip
	case "snappy":
		return kafka.Snappy
	case "lz4":
		return kafka.Lz4
	case "zstd":
		return kafka.Zstd
	default:
		return kafka.Compression(0)
	}
}
