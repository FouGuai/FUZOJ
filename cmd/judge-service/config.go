package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	"fuzoj/judge_service/internal/sandbox/engine"
	"fuzoj/judge_service/internal/sandbox/profile"
	"fuzoj/pkg/utils/logger"

	"github.com/segmentio/kafka-go"
	"gopkg.in/yaml.v3"
)

const (
	defaultHTTPAddr        = "0.0.0.0:8085"
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultIdleTimeout     = 60 * time.Second
	defaultShutdownTimeout = 10 * time.Second
	defaultMetaTTL         = 30 * time.Second
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"readTimeout"`
	WriteTimeout time.Duration `yaml:"writeTimeout"`
	IdleTimeout  time.Duration `yaml:"idleTimeout"`
}

// KafkaConfig holds Kafka settings.
type KafkaConfig struct {
	Brokers       []string       `yaml:"brokers"`
	ClientID      string         `yaml:"clientID"`
	MinBytes      int            `yaml:"minBytes"`
	MaxBytes      int            `yaml:"maxBytes"`
	MaxWait       time.Duration  `yaml:"maxWait"`
	BatchSize     int            `yaml:"batchSize"`
	BatchTimeout  time.Duration  `yaml:"batchTimeout"`
	DialTimeout   time.Duration  `yaml:"dialTimeout"`
	ReadTimeout   time.Duration  `yaml:"readTimeout"`
	WriteTimeout  time.Duration  `yaml:"writeTimeout"`
	RequiredAcks  int            `yaml:"requiredAcks"`
	Compression   string         `yaml:"compression"`
	Topics        []string       `yaml:"topics"`
	ConsumerGroup string         `yaml:"consumerGroup"`
	PrefetchCount int            `yaml:"prefetchCount"`
	Concurrency   int            `yaml:"concurrency"`
	MaxRetries    int            `yaml:"maxRetries"`
	RetryDelay    time.Duration  `yaml:"retryDelay"`
	RetryTopic    string         `yaml:"retryTopic"`
	PoolRetryMax  int            `yaml:"poolRetryMax"`
	PoolRetryBase time.Duration  `yaml:"poolRetryBaseDelay"`
	PoolRetryMaxD time.Duration  `yaml:"poolRetryMaxDelay"`
	DeadLetter    string         `yaml:"deadLetterTopic"`
	MessageTTL    time.Duration  `yaml:"messageTTL"`
	TopicWeights  map[string]int `yaml:"topicWeights"`
}

// CacheConfig holds local cache settings.
type CacheConfig struct {
	RootDir    string        `yaml:"rootDir"`
	TTL        time.Duration `yaml:"ttl"`
	LockWait   time.Duration `yaml:"lockWait"`
	MaxEntries int           `yaml:"maxEntries"`
	MaxBytes   int64         `yaml:"maxBytes"`
}

// WorkerConfig holds worker pool settings.
type WorkerConfig struct {
	PoolSize int           `yaml:"poolSize"`
	Timeout  time.Duration `yaml:"timeout"`
}

// SourceConfig holds source download settings.
type SourceConfig struct {
	Bucket  string        `yaml:"bucket"`
	Timeout time.Duration `yaml:"timeout"`
}

// ProblemRPCConfig holds problem service gRPC settings.
type ProblemRPCConfig struct {
	Addr    string        `yaml:"addr"`
	Timeout time.Duration `yaml:"timeout"`
	MetaTTL time.Duration `yaml:"metaTTL"`
}

// StatusConfig holds status persistence settings.
type StatusConfig struct {
	TTL        time.Duration `yaml:"ttl"`
	Timeout    time.Duration `yaml:"timeout"`
	FinalTopic string        `yaml:"finalTopic"`
}

// JudgeConfig holds judge work settings.
type JudgeConfig struct {
	WorkRoot string `yaml:"workRoot"`
}

// SandboxConfig holds sandbox engine settings.
type SandboxConfig struct {
	CgroupRoot           string `yaml:"cgroupRoot"`
	SeccompDir           string `yaml:"seccompDir"`
	HelperPath           string `yaml:"helperPath"`
	StdoutStderrMaxBytes int64  `yaml:"stdoutStderrMaxBytes"`
	EnableSeccomp        bool   `yaml:"enableSeccomp"`
	EnableCgroup         bool   `yaml:"enableCgroup"`
	EnableNamespaces     bool   `yaml:"enableNamespaces"`
}

// LanguageConfig holds language definitions.
type LanguageConfig struct {
	Languages []profile.LanguageSpec `yaml:"languages"`
	Profiles  []profile.TaskProfile  `yaml:"profiles"`
}

// AppConfig holds judge-service config.
type AppConfig struct {
	Server   ServerConfig        `yaml:"server"`
	Logger   logger.Config       `yaml:"logger"`
	Kafka    KafkaConfig         `yaml:"kafka"`
	Database db.MySQLConfig      `yaml:"database"`
	Redis    cache.RedisConfig   `yaml:"redis"`
	MinIO    storage.MinIOConfig `yaml:"minio"`
	Problem  ProblemRPCConfig    `yaml:"problemRPC"`
	Cache    CacheConfig         `yaml:"cache"`
	Worker   WorkerConfig        `yaml:"worker"`
	Source   SourceConfig        `yaml:"source"`
	Status   StatusConfig        `yaml:"status"`
	Judge    JudgeConfig         `yaml:"judge"`
	Sandbox  SandboxConfig       `yaml:"sandbox"`
	Language LanguageConfig      `yaml:"language"`
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
	if cfg.Database.DSN == "" {
		return nil, fmt.Errorf("database dsn is required")
	}
	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}
	applyRedisDefaults(&cfg.Redis)
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
	if cfg.Problem.MetaTTL == 0 {
		cfg.Problem.MetaTTL = defaultMetaTTL
	}
	if cfg.Source.Bucket == "" {
		cfg.Source.Bucket = cfg.MinIO.Bucket
	}
	if cfg.Worker.PoolSize <= 0 {
		cfg.Worker.PoolSize = 1
	}
	if cfg.Status.FinalTopic == "" {
		cfg.Status.FinalTopic = "judge.status.final"
	}
	if cfg.Kafka.RetryTopic == "" {
		cfg.Kafka.RetryTopic = "judge.retry"
	}
	if cfg.Kafka.PoolRetryMax <= 0 {
		cfg.Kafka.PoolRetryMax = 5
	}
	if cfg.Kafka.PoolRetryBase == 0 {
		cfg.Kafka.PoolRetryBase = time.Second
	}
	if cfg.Kafka.PoolRetryMaxD == 0 {
		cfg.Kafka.PoolRetryMaxD = 30 * time.Second
	}
	if len(cfg.Kafka.TopicWeights) == 0 && len(cfg.Kafka.Topics) > 0 {
		cfg.Kafka.TopicWeights = defaultTopicWeights(cfg.Kafka.Topics)
	}
	return &cfg, nil
}

func defaultTopicWeights(topics []string) map[string]int {
	weights := []int{8, 4, 2, 1}
	out := make(map[string]int, len(topics))
	for i, topic := range topics {
		if topic == "" {
			continue
		}
		if i < len(weights) {
			out[topic] = weights[i]
			continue
		}
		out[topic] = 1
	}
	return out
}

func applyRedisDefaults(cfg *cache.RedisConfig) {
	if cfg == nil {
		return
	}
	defaults := cache.DefaultRedisConfig()
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = defaults.MinRetryBackoff
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = defaults.MaxRetryBackoff
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = defaults.DialTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = defaults.ReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaults.WriteTimeout
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = defaults.PoolSize
	}
	if cfg.MinIdleConns == 0 {
		cfg.MinIdleConns = defaults.MinIdleConns
	}
	if cfg.PoolTimeout == 0 {
		cfg.PoolTimeout = defaults.PoolTimeout
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = defaults.ConnMaxIdleTime
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = defaults.ConnMaxLifetime
	}
}

func (k KafkaConfig) toMQConfig() mq.KafkaConfig {
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

func (s SandboxConfig) toEngineConfig() engine.Config {
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
