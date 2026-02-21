package model

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserBansModel = (*customUserBansModel)(nil)

type (
	// UserBansModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserBansModel.
	UserBansModel interface {
		userBansModel
	}

	customUserBansModel struct {
		*defaultUserBansModel
	}
)

// NewUserBansModel returns a model for the database table.
func NewUserBansModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserBansModel {
	return &customUserBansModel{
		defaultUserBansModel: newUserBansModel(conn, c, opts...),
	}
}
