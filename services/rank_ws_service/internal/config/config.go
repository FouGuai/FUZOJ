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
	Cache     cache.CacheConf  `json:"cache"`
	Redis     redis.RedisConf  `json:"redis"`
	Rank      RankConfig       `json:"rank"`
	Timeouts  TimeoutConfig    `json:"timeouts"`
}

type RankConfig struct {
	PageCacheTTL time.Duration `json:"pageCacheTTL"`
	EmptyTTL     time.Duration `json:"emptyTTL"`
	WSDebounce   time.Duration `json:"wsDebounce"`
}

type TimeoutConfig struct {
	Cache time.Duration `json:"cache"`
	WS    time.Duration `json:"ws"`
}
