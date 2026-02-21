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
	Cache cache.CacheConf `json:"cache"`
	Redis redis.RedisConf `json:"redis"`
	Auth  AuthConfig      `json:"auth"`
}

type AuthConfig struct {
	JWTSecret       string            `json:"jwtSecret"`
	JWTIssuer       string            `json:"jwtIssuer"`
	AccessTokenTTL  time.Duration     `json:"accessTokenTTL"`
	RefreshTokenTTL time.Duration     `json:"refreshTokenTTL"`
	LoginFailTTL    time.Duration     `json:"loginFailTTL"`
	LoginFailLimit  int               `json:"loginFailLimit"`
	Root            RootAccountConfig `json:"root"`
}

type RootAccountConfig struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}
