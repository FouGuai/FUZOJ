// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"fuzoj/services/user_service/internal/config"
	"fuzoj/services/user_service/internal/model"
	"fuzoj/services/user_service/internal/repository"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config          config.Config
	Conn            sqlx.SqlConn
	UsersModel      model.UsersModel
	UserBansModel   model.UserBansModel
	UserTokensModel model.UserTokensModel
	Redis           *redis.Redis
	BanCacheRepo    repository.BanCacheRepository
	UserRepo        repository.UserRepository
	TokenRepo       repository.TokenRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	redisClient := redis.MustNewRedis(c.Redis)
	tokensModel := model.NewUserTokensModel(conn, c.Cache)
	usersModel := model.NewUsersModel(conn, c.Cache)
	userRepo := repository.NewUserRepository(usersModel)
	tokenRepo := repository.NewTokenRepository(tokensModel, redisClient)
	return &ServiceContext{
		Config:          c,
		Conn:            conn,
		UsersModel:      usersModel,
		UserBansModel:   model.NewUserBansModel(conn, c.Cache),
		UserTokensModel: tokensModel,
		Redis:           redisClient,
		BanCacheRepo:    repository.NewBanCacheRepository(redisClient),
		UserRepo:        userRepo,
		TokenRepo:       tokenRepo,
	}
}
