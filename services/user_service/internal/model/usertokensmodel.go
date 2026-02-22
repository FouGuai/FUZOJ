package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserTokensModel = (*customUserTokensModel)(nil)

type (
	// UserTokensModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserTokensModel.
	UserTokensModel interface {
		userTokensModel
		FindActiveTokensByUserID(ctx context.Context, userID int64, now time.Time) ([]UserTokenBrief, error)
		RevokeActiveByUserID(ctx context.Context, userID int64, now time.Time) ([]UserTokenBrief, error)
		WithSession(session sqlx.Session) UserTokensModel
	}

	customUserTokensModel struct {
		*defaultUserTokensModel
	}
)

// NewUserTokensModel returns a model for the database table.
func NewUserTokensModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserTokensModel {
	return &customUserTokensModel{
		defaultUserTokensModel: newUserTokensModel(conn, c, opts...),
	}
}

func (m *customUserTokensModel) WithSession(session sqlx.Session) UserTokensModel {
	if session == nil {
		return m
	}
	return &customUserTokensModel{
		defaultUserTokensModel: &defaultUserTokensModel{
			CachedConn: m.CachedConn.WithSession(session),
			table:      m.table,
		},
	}
}

type UserTokenBrief struct {
	Id        int64     `db:"id"`
	TokenHash string    `db:"token_hash"`
	ExpiresAt time.Time `db:"expires_at"`
}

func (m *customUserTokensModel) FindActiveTokensByUserID(ctx context.Context, userID int64, now time.Time) ([]UserTokenBrief, error) {
	records, err := m.findTokensByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	activeTokens := make([]UserTokenBrief, 0, len(records))
	for _, token := range records {
		if token.Revoked || !token.ExpiresAt.After(now) {
			continue
		}
		activeTokens = append(activeTokens, UserTokenBrief{
			Id:        token.Id,
			TokenHash: token.TokenHash,
			ExpiresAt: token.ExpiresAt,
		})
	}

	return activeTokens, nil
}

func (m *customUserTokensModel) RevokeActiveByUserID(ctx context.Context, userID int64, now time.Time) ([]UserTokenBrief, error) {
	records, err := m.findTokensByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	activeTokens := make([]UserTokenBrief, 0, len(records))
	keys := make([]string, 0, len(records)*2)
	for _, token := range records {
		keys = append(keys, fmt.Sprintf("%s%v", cacheUserTokensIdPrefix, token.Id))
		keys = append(keys, fmt.Sprintf("%s%v", cacheUserTokensTokenHashPrefix, token.TokenHash))
		if token.Revoked || !token.ExpiresAt.After(now) {
			continue
		}
		activeTokens = append(activeTokens, UserTokenBrief{
			Id:        token.Id,
			TokenHash: token.TokenHash,
			ExpiresAt: token.ExpiresAt,
		})
	}

	query := fmt.Sprintf("update %s set `revoked` = true where `user_id` = ?", m.table)
	if _, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, userID)
	}, keys...); err != nil {
		return nil, err
	}

	return activeTokens, nil
}

type userTokenRecord struct {
	Id        int64     `db:"id"`
	TokenHash string    `db:"token_hash"`
	ExpiresAt time.Time `db:"expires_at"`
	Revoked   bool      `db:"revoked"`
}

func (m *customUserTokensModel) findTokensByUserID(ctx context.Context, userID int64) ([]userTokenRecord, error) {
	query := fmt.Sprintf("select `id`, `token_hash`, `expires_at`, `revoked` from %s where `user_id` = ?", m.table)
	var tokens []userTokenRecord
	if err := m.QueryRowsNoCacheCtx(ctx, &tokens, query, userID); err != nil {
		return nil, err
	}
	return tokens, nil
}
