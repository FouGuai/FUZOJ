// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"time"

	"fuzoj/pkg/bootstrap"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Bootstrap bootstrap.Config `json:"bootstrap,optional"`
	Mysql     struct {
		DataSource string `json:"dataSource"`
	} `json:"mysql"`
	Cache     cache.CacheConf `json:"cache"`
	RankRedis redis.RedisConf `json:"rankRedis"`
	Rank      RankConfig      `json:"rank"`
	Timeouts  TimeoutConfig   `json:"timeouts"`
}

type RankConfig struct {
	HotCacheTTL  time.Duration `json:"hotCacheTTL"`
	PageCacheTTL time.Duration `json:"pageCacheTTL"`
	EmptyTTL     time.Duration `json:"emptyTTL"`
}

type TimeoutConfig struct {
	Cache time.Duration `json:"cache"`
}
