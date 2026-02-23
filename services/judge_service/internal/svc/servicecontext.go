// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"fuzoj/internal/common/storage"
	"fuzoj/services/judge_service/internal/cache"
	"fuzoj/services/judge_service/internal/config"
	"fuzoj/services/judge_service/internal/model"
	"fuzoj/services/judge_service/internal/problemclient"
	"fuzoj/services/judge_service/internal/repository"
	"fuzoj/services/judge_service/internal/sandbox"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config           config.Config
	Conn             sqlx.SqlConn
	SubmissionsModel model.SubmissionsModel
	StatusCache      *redis.Redis
	StatusPublisher  repository.StatusEventPublisher
	StatusRepo       *repository.StatusRepository
	Worker           *sandbox.Worker
	ProblemClient    *problemclient.Client
	DataCache        *cache.DataPackCache
	Storage          storage.ObjectStorage
	StatusPusher     *kq.Pusher
	RetryPusher      *kq.Pusher
	DeadLetterPusher *kq.Pusher
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

func newStatusCache(c config.Config) *redis.Redis {
	if c.Redis.Host == "" {
		return nil
	}
	client, err := redis.NewRedis(c.Redis)
	if err != nil {
		logx.Errorf("init status cache failed: %v", err)
		return nil
	}
	return client
}
