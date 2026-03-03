// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"fuzoj/internal/common/storage"
	"fuzoj/services/status_service/internal/config"
	"fuzoj/services/status_service/internal/model"
	"fuzoj/services/status_service/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config           config.Config
	Conn             sqlx.SqlConn
	Redis            *redis.Redis
	SubmissionsModel model.SubmissionsModel
	StatusRepo       *repository.StatusRepository
	LogRepo          *repository.SubmissionLogRepository
	Storage          storage.ObjectStorage
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	redisClient := redis.MustNewRedis(c.Redis)

	submissionsModel := model.NewSubmissionsModel(conn, c.Cache)
	statusRepo := repository.NewStatusRepository(redisClient, submissionsModel, c.Status.CacheTTL, c.Status.CacheEmptyTTL)

	var storageClient storage.ObjectStorage
	if c.MinIO.Endpoint != "" {
		st, err := storage.NewMinIOStorage(storage.MinIOConfig{
			Endpoint:  c.MinIO.Endpoint,
			AccessKey: c.MinIO.AccessKey,
			SecretKey: c.MinIO.SecretKey,
			UseSSL:    c.MinIO.UseSSL,
		})
		if err == nil {
			storageClient = st
		} else {
			logx.Errorf("init minio storage failed: %v", err)
		}
	}
	logBucket := c.Status.LogBucket
	if logBucket == "" {
		logBucket = c.MinIO.Bucket
	}
	logRepo := repository.NewSubmissionLogRepository(
		conn,
		redisClient,
		storageClient,
		logBucket,
		c.Status.LogKeyPrefix,
		c.Status.LogMaxBytes,
		c.Status.LogCacheTTL,
	)

	return &ServiceContext{
		Config:           c,
		Conn:             conn,
		Redis:            redisClient,
		SubmissionsModel: submissionsModel,
		StatusRepo:       statusRepo,
		LogRepo:          logRepo,
		Storage:          storageClient,
	}
}
