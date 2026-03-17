package svc

import (
	"fuzoj/services/status_sse_service/internal/config"
	"fuzoj/services/status_sse_service/internal/repository"
	"fuzoj/services/status_sse_service/internal/sse"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config       config.Config
	Conn         sqlx.SqlConn
	Redis        *redis.Redis
	PubSubClient *red.Client
	StatusRepo   *repository.StatusRepository
	Hub          *sse.Hub
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	redisClient := redis.MustNewRedis(c.Redis)
	pubsubClient := newPubSubClient(c.Redis)
	repo := repository.NewStatusRepository(
		conn,
		redisClient,
		c.StatusSSE.OwnerCacheTTL,
		c.StatusSSE.OwnerCacheEmptyTTL,
	)
	hub := sse.NewHub(repo, pubsubClient, c.StatusSSE.Debounce, c.StatusSSE.Heartbeat)

	return &ServiceContext{
		Config:       c,
		Conn:         conn,
		Redis:        redisClient,
		PubSubClient: pubsubClient,
		StatusRepo:   repo,
		Hub:          hub,
	}
}

func newPubSubClient(conf redis.RedisConf) *red.Client {
	if conf.Host == "" {
		return nil
	}
	if conf.Type != "" && conf.Type != "node" {
		logx.Errorf("redis pubsub only supports node type, got %s", conf.Type)
		return nil
	}
	opt := &red.Options{
		Addr:     conf.Host,
		Username: conf.User,
		Password: conf.Pass,
	}
	return red.NewClient(opt)
}
