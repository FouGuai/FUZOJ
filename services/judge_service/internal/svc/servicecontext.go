// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	cachex "fuzoj/internal/common/cache"
	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	"fuzoj/services/judge_service/internal/cache"
	"fuzoj/services/judge_service/internal/config"
	"fuzoj/services/judge_service/internal/model"
	"fuzoj/services/judge_service/internal/problemclient"
	"fuzoj/services/judge_service/internal/repository"
	"fuzoj/services/judge_service/internal/sandbox"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config           config.Config
	Conn             sqlx.SqlConn
	SubmissionsModel model.SubmissionsModel
	StatusCache      cachex.Cache
	StatusPublisher  repository.StatusEventPublisher
	StatusRepo       *repository.StatusRepository
	Worker           *sandbox.Worker
	ProblemClient    *problemclient.Client
	DataCache        *cache.DataPackCache
	Storage          storage.ObjectStorage
	Queue            mq.MessageQueue
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	statusCache := newStatusCache(c)
	submissionsModel := model.NewSubmissionsModel(conn, c.Cache)
	statusRepo := repository.NewStatusRepository(
		statusCache,
		submissionsModel,
		c.StatusCacheTTL,
		c.StatusCacheEmptyTTL,
		nil,
	)
	return &ServiceContext{
		Config:           c,
		Conn:             conn,
		SubmissionsModel: submissionsModel,
		StatusCache:      statusCache,
		StatusRepo:       statusRepo,
	}
}

func newStatusCache(c config.Config) cachex.Cache {
	if c.Redis.Host == "" {
		return nil
	}
	redisConfig := cachex.DefaultRedisConfig()
	redisConfig.Addr = c.Redis.Host
	redisConfig.Password = c.Redis.Pass
	redisConfig.DialTimeout = c.Redis.PingTimeout
	cacheClient, err := cachex.NewRedisCacheWithConfig(redisConfig)
	if err != nil {
		logx.Errorf("init status cache failed: %v", err)
		return nil
	}
	return cacheClient
}
