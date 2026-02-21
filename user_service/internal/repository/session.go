package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"fuzoj/user_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type UserToken struct {
	ID         int64
	UserID     int64
	TokenHash  string
	TokenType  TokenType
	DeviceInfo string
	IPAddress  string
	ExpiresAt  time.Time
	Revoked    bool
	CreatedAt  time.Time
}

type TokenRepository interface {
	Create(ctx context.Context, token *UserToken) error
	GetByHash(ctx context.Context, tokenHash string) (*UserToken, error)
	RevokeByHash(ctx context.Context, tokenHash string, expiresAt time.Time) error
	RevokeByUser(ctx context.Context, userID int64) error
	IsBlacklisted(ctx context.Context, tokenHash string) (bool, error)
	WithSession(session sqlx.Session) TokenRepository
}

type MySQLTokenRepository struct {
	model model.UserTokensModel
	redis *redis.Redis
}

func NewTokenRepository(tokensModel model.UserTokensModel, redisClient *redis.Redis) TokenRepository {
	return &MySQLTokenRepository{
		model: tokensModel,
		redis: redisClient,
	}
}

func (r *MySQLTokenRepository) WithSession(session sqlx.Session) TokenRepository {
	if session == nil {
		return r
	}
	return &MySQLTokenRepository{
		model: r.model.WithSession(session),
		redis: r.redis,
	}
}

func (r *MySQLTokenRepository) Create(ctx context.Context, token *UserToken) error {
	if token == nil {
		return errors.New("token is nil")
	}
	deviceInfo := sql.NullString{}
	if token.DeviceInfo != "" {
		deviceInfo = sql.NullString{String: token.DeviceInfo, Valid: true}
	}
	ipAddress := sql.NullString{}
	if token.IPAddress != "" {
		ipAddress = sql.NullString{String: token.IPAddress, Valid: true}
	}

	_, err := r.model.Insert(ctx, &model.UserTokens{
		UserId:     token.UserID,
		TokenHash:  token.TokenHash,
		TokenType:  string(token.TokenType),
		DeviceInfo: deviceInfo,
		IpAddress:  ipAddress,
		ExpiresAt:  token.ExpiresAt,
		Revoked:    token.Revoked,
	})
	return err
}

func (r *MySQLTokenRepository) GetByHash(ctx context.Context, tokenHash string) (*UserToken, error) {
	result, err := r.model.FindOneByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return fromTokenModel(result), nil
}

func (r *MySQLTokenRepository) RevokeByHash(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	token, err := r.model.FindOneByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ErrTokenNotFound
		}
		return err
	}
	if !token.Revoked {
		token.Revoked = true
		if err := r.model.Update(ctx, token); err != nil {
			return err
		}
	}
	return r.blacklistToken(ctx, tokenHash, expiresAt)
}

func (r *MySQLTokenRepository) RevokeByUser(ctx context.Context, userID int64) error {
	now := time.Now()
	tokens, err := r.model.RevokeActiveByUserID(ctx, userID, now)
	if err != nil {
		return err
	}

	for _, token := range tokens {
		if err := r.blacklistToken(ctx, token.TokenHash, token.ExpiresAt); err != nil {
			return err
		}
	}

	return nil
}

func (r *MySQLTokenRepository) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	if r.redis == nil {
		return false, errors.New("cache is nil")
	}
	return r.redis.SismemberCtx(ctx, tokenBlacklistKey, tokenHash)
}

func (r *MySQLTokenRepository) blacklistToken(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	if r.redis == nil {
		return errors.New("cache is nil")
	}

	if _, err := r.redis.SaddCtx(ctx, tokenBlacklistKey, tokenHash); err != nil {
		return err
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}

	return extendRedisTTL(ctx, r.redis, tokenBlacklistKey, ttl)
}

func fromTokenModel(token *model.UserTokens) *UserToken {
	if token == nil {
		return nil
	}
	result := &UserToken{
		ID:        token.Id,
		UserID:    token.UserId,
		TokenHash: token.TokenHash,
		TokenType: TokenType(token.TokenType),
		ExpiresAt: token.ExpiresAt,
		Revoked:   token.Revoked,
		CreatedAt: token.CreatedAt,
	}
	if token.DeviceInfo.Valid {
		result.DeviceInfo = token.DeviceInfo.String
	}
	if token.IpAddress.Valid {
		result.IPAddress = token.IpAddress.String
	}
	return result
}
