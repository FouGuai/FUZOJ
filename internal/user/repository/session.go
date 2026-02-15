package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
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
	Create(ctx context.Context, tx db.Transaction, token *UserToken) error
	GetByHash(ctx context.Context, tx db.Transaction, tokenHash string) (*UserToken, error)
	RevokeByHash(ctx context.Context, tx db.Transaction, tokenHash string, expiresAt time.Time) error
	RevokeByUser(ctx context.Context, tx db.Transaction, userID int64) error
	IsBlacklisted(ctx context.Context, tokenHash string) (bool, error)
}

type MySQLTokenRepository struct {
	dbProvider db.Provider
	cache      cache.Cache
	ttl        time.Duration
	emptyTTL   time.Duration
}

func NewTokenRepository(provider db.Provider, cacheClient cache.Cache) TokenRepository {
	return NewTokenRepositoryWithTTL(provider, cacheClient, defaultTokenCacheTTL, defaultTokenCacheEmptyTTL)
}

func NewTokenRepositoryWithTTL(provider db.Provider, cacheClient cache.Cache, ttl, emptyTTL time.Duration) TokenRepository {
	if ttl <= 0 {
		ttl = defaultTokenCacheTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultTokenCacheEmptyTTL
	}
	return &MySQLTokenRepository{dbProvider: provider, cache: cacheClient, ttl: ttl, emptyTTL: emptyTTL}
}

const tokenColumns = "id, user_id, token_hash, token_type, device_info, ip_address, expires_at, revoked, created_at"
const (
	tokenCacheKeyPrefix       = "token:hash:"
	defaultTokenCacheTTL      = 30 * time.Minute
	defaultTokenCacheEmptyTTL = 5 * time.Minute
)

func (r *MySQLTokenRepository) Create(ctx context.Context, tx db.Transaction, token *UserToken) error {
	if token == nil {
		return errors.New("token is nil")
	}

	query := "INSERT INTO user_tokens (user_id, token_hash, token_type, device_info, ip_address, expires_at, revoked) VALUES (?, ?, ?, ?, ?, ?, ?)"
	deviceInfo := sql.NullString{}
	if token.DeviceInfo != "" {
		deviceInfo = sql.NullString{String: token.DeviceInfo, Valid: true}
	}
	ipAddress := sql.NullString{}
	if token.IPAddress != "" {
		ipAddress = sql.NullString{String: token.IPAddress, Valid: true}
	}

	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return err
	}
	_, err = querier.Exec(
		ctx,
		query,
		token.UserID,
		token.TokenHash,
		token.TokenType,
		deviceInfo,
		ipAddress,
		token.ExpiresAt,
		token.Revoked,
	)
	if err != nil {
		return err
	}
	if r.cache != nil && tx == nil {
		r.setCache(ctx, token)
	}
	return nil
}

func (r *MySQLTokenRepository) GetByHash(ctx context.Context, tx db.Transaction, tokenHash string) (*UserToken, error) {
	if r.cache != nil && tx == nil {
		key := tokenCacheKey(tokenHash)
		if cached, err := r.cache.Get(ctx, key); err == nil && cached != "" {
			if cached == cache.NullCacheValue {
				return nil, ErrTokenNotFound
			}
			token, err := unmarshalToken(cached)
			if err == nil && token != nil {
				return token, nil
			}
		}

		token, err := r.getByHashFromDB(ctx, nil, tokenHash)
		if err != nil {
			if errors.Is(err, ErrTokenNotFound) {
				_ = r.cache.Set(ctx, key, cache.NullCacheValue, cache.JitterTTL(r.emptyTTL))
				return nil, ErrTokenNotFound
			}
			return nil, err
		}
		r.setCache(ctx, token)
		return token, nil
	}

	return r.getByHashFromDB(ctx, tx, tokenHash)
}

func (r *MySQLTokenRepository) RevokeByHash(ctx context.Context, tx db.Transaction, tokenHash string, expiresAt time.Time) error {
	query := "UPDATE user_tokens SET revoked = TRUE WHERE token_hash = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return err
	}
	result, err := querier.Exec(ctx, query, tokenHash)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTokenNotFound
	}
	if r.cache != nil {
		r.deleteCache(ctx, tokenHash)
	}

	return r.blacklistToken(ctx, tokenHash, expiresAt)
}

func (r *MySQLTokenRepository) RevokeByUser(ctx context.Context, tx db.Transaction, userID int64) error {
	now := time.Now()
	queryTokens := "SELECT token_hash, expires_at FROM user_tokens WHERE user_id = ? AND revoked = FALSE AND expires_at > ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return err
	}
	rows, err := querier.Query(ctx, queryTokens, userID, now)
	if err != nil {
		return err
	}
	defer rows.Close()

	tokens := make([]UserToken, 0, 8)
	for rows.Next() {
		var token UserToken
		if err := rows.Scan(&token.TokenHash, &token.ExpiresAt); err != nil {
			return err
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	queryRevoke := "UPDATE user_tokens SET revoked = TRUE WHERE user_id = ?"
	if _, err := querier.Exec(ctx, queryRevoke, userID); err != nil {
		return err
	}

	for _, token := range tokens {
		if err := r.blacklistToken(ctx, token.TokenHash, token.ExpiresAt); err != nil {
			return err
		}
		if r.cache != nil {
			r.deleteCache(ctx, token.TokenHash)
		}
	}

	return nil
}

func (r *MySQLTokenRepository) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	if r.cache == nil {
		return false, errors.New("cache is nil")
	}
	return r.cache.SIsMember(ctx, tokenBlacklistKey, tokenHash)
}

func (r *MySQLTokenRepository) blacklistToken(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	if r.cache == nil {
		return errors.New("cache is nil")
	}

	if err := r.cache.SAdd(ctx, tokenBlacklistKey, tokenHash); err != nil {
		return err
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}

	return extendTTL(ctx, r.cache, tokenBlacklistKey, ttl)
}

func (r *MySQLTokenRepository) getByHashFromDB(ctx context.Context, tx db.Transaction, tokenHash string) (*UserToken, error) {
	query := "SELECT " + tokenColumns + " FROM user_tokens WHERE token_hash = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return nil, err
	}
	row := querier.QueryRow(ctx, query, tokenHash)
	result, err := scanToken(row)
	if err != nil {
		if db.IsNoRows(err) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return result, nil
}

func (r *MySQLTokenRepository) cacheTTL(expiresAt time.Time) time.Duration {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return 0
	}
	if r.ttl > 0 && ttl > r.ttl {
		ttl = r.ttl
	}
	return cache.JitterTTL(ttl)
}

func (r *MySQLTokenRepository) setCache(ctx context.Context, token *UserToken) {
	if r.cache == nil || token == nil {
		return
	}
	ttl := r.cacheTTL(token.ExpiresAt)
	if ttl <= 0 {
		return
	}
	payload, err := json.Marshal(token)
	if err != nil {
		return
	}
	_ = r.cache.Set(ctx, tokenCacheKey(token.TokenHash), string(payload), ttl)
}

func (r *MySQLTokenRepository) deleteCache(ctx context.Context, tokenHash string) {
	if r.cache == nil || tokenHash == "" {
		return
	}
	_ = r.cache.Del(ctx, tokenCacheKey(tokenHash))
}

func tokenCacheKey(tokenHash string) string {
	return fmt.Sprintf("%s%s", tokenCacheKeyPrefix, tokenHash)
}

func unmarshalToken(data string) (*UserToken, error) {
	if data == "" {
		return nil, nil
	}
	var token UserToken
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func scanToken(scanner db.Scanner) (*UserToken, error) {
	var token UserToken
	var deviceInfo sql.NullString
	var ipAddress sql.NullString

	err := scanner.Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.TokenType,
		&deviceInfo,
		&ipAddress,
		&token.ExpiresAt,
		&token.Revoked,
		&token.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if deviceInfo.Valid {
		token.DeviceInfo = deviceInfo.String
	}
	if ipAddress.Valid {
		token.IPAddress = ipAddress.String
	}

	return &token, nil
}
