// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	cachex "fuzoj/user_service/internal/cache"
	"fuzoj/user_service/internal/config"
	"fuzoj/user_service/internal/model"
	"fuzoj/user_service/internal/repository"
	"fuzoj/user_service/internal/service"

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
	AuthService     *service.AuthService
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	redisClient := redis.MustNewRedis(c.Redis)
	tokensModel := model.NewUserTokensModel(conn, c.Cache)
	usersModel := model.NewUsersModel(conn, c.Cache)
	userRepo := repository.NewUserRepository(usersModel)
	tokenRepo := repository.NewTokenRepository(tokensModel, redisClient)
	loginFailCache := cachex.NewRedisBasicCache(redisClient)
	authService := service.NewAuthService(
		conn,
		userRepo,
		tokenRepo,
		loginFailCache,
		service.AuthServiceConfig{
			JWTSecret:       []byte(c.Auth.JWTSecret),
			JWTIssuer:       c.Auth.JWTIssuer,
			AccessTokenTTL:  c.Auth.AccessTokenTTL,
			RefreshTokenTTL: c.Auth.RefreshTokenTTL,
			LoginFailTTL:    c.Auth.LoginFailTTL,
			LoginFailLimit:  c.Auth.LoginFailLimit,
			Root: service.RootAccountConfig{
				Enabled:  c.Auth.Root.Enabled,
				Username: c.Auth.Root.Username,
				Password: c.Auth.Root.Password,
				Email:    c.Auth.Root.Email,
			},
		},
	)
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
		AuthService:     authService,
	}
}
