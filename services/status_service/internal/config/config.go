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
	Cache  cache.CacheConf `json:"cache"`
	Redis  redis.RedisConf `json:"redis"`
	MinIO  MinIOConfig     `json:"minio"`
	Status StatusConfig    `json:"status"`
}

type MinIOConfig struct {
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	UseSSL    bool   `json:"useSSL"`
	Bucket    string `json:"bucket"`
}

type StatusConfig struct {
	CacheTTL      time.Duration `json:"cacheTTL"`
	CacheEmptyTTL time.Duration `json:"cacheEmptyTTL"`
	LogBucket     string        `json:"logBucket"`
	LogKeyPrefix  string        `json:"logKeyPrefix"`
	LogMaxBytes   int           `json:"logMaxBytes"`
	LogCacheTTL   time.Duration `json:"logCacheTTL"`
	Timeouts      TimeoutConfig `json:"timeouts"`
}

type TimeoutConfig struct {
	DB      time.Duration `json:"db"`
	Cache   time.Duration `json:"cache"`
	Storage time.Duration `json:"storage"`
	Status  time.Duration `json:"status"`
}
