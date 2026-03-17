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
	Cache     cache.CacheConf `json:"cache"`
	Redis     redis.RedisConf `json:"redis"`
	StatusSSE StatusSSEConfig `json:"statusSSE"`
	Timeouts  TimeoutConfig   `json:"timeouts"`
}

type StatusSSEConfig struct {
	OwnerCacheTTL      time.Duration `json:"ownerCacheTTL"`
	OwnerCacheEmptyTTL time.Duration `json:"ownerCacheEmptyTTL"`
	Debounce           time.Duration `json:"debounce"`
	Heartbeat          time.Duration `json:"heartbeat"`
}

type TimeoutConfig struct {
	DB    time.Duration `json:"db"`
	Cache time.Duration `json:"cache"`
	SSE   time.Duration `json:"sse"`
}
