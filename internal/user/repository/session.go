package repository

import (
	"context"
	"database/sql"
	"errors"
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
	DeviceInfo *string
	IPAddress  *string
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

type PostgresTokenRepository struct {
	db    db.Database
	cache cache.Cache
}

func NewTokenRepository(database db.Database, cacheClient cache.Cache) TokenRepository {
	return &PostgresTokenRepository{db: database, cache: cacheClient}
}

const tokenColumns = "id, user_id, token_hash, token_type, device_info, ip_address, expires_at, revoked, created_at"

func (r *PostgresTokenRepository) Create(ctx context.Context, tx db.Transaction, token *UserToken) error {
	if token == nil {
		return errors.New("token is nil")
	}

	query := "INSERT INTO user_tokens (user_id, token_hash, token_type, device_info, ip_address, expires_at, revoked) VALUES ($1, $2, $3, $4, $5, $6, $7)"
	deviceInfo := sql.NullString{}
	if token.DeviceInfo != nil {
		deviceInfo = sql.NullString{String: *token.DeviceInfo, Valid: true}
	}
	ipAddress := sql.NullString{}
	if token.IPAddress != nil {
		ipAddress = sql.NullString{String: *token.IPAddress, Valid: true}
	}

	_, err := getQuerier(r.db, tx).Exec(
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
	return err
}

func (r *PostgresTokenRepository) GetByHash(ctx context.Context, tx db.Transaction, tokenHash string) (*UserToken, error) {
	query := "SELECT " + tokenColumns + " FROM user_tokens WHERE token_hash = $1"
	row := getQuerier(r.db, tx).QueryRow(ctx, query, tokenHash)
	result, err := scanToken(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return result, nil
}

func (r *PostgresTokenRepository) RevokeByHash(ctx context.Context, tx db.Transaction, tokenHash string, expiresAt time.Time) error {
	query := "UPDATE user_tokens SET revoked = TRUE WHERE token_hash = $1"
	result, err := getQuerier(r.db, tx).Exec(ctx, query, tokenHash)
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

	return r.blacklistToken(ctx, tokenHash, expiresAt)
}

func (r *PostgresTokenRepository) RevokeByUser(ctx context.Context, tx db.Transaction, userID int64) error {
	now := time.Now()
	queryTokens := "SELECT token_hash, expires_at FROM user_tokens WHERE user_id = $1 AND revoked = FALSE AND expires_at > $2"
	rows, err := getQuerier(r.db, tx).Query(ctx, queryTokens, userID, now)
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

	queryRevoke := "UPDATE user_tokens SET revoked = TRUE WHERE user_id = $1"
	if _, err := getQuerier(r.db, tx).Exec(ctx, queryRevoke, userID); err != nil {
		return err
	}

	for _, token := range tokens {
		if err := r.blacklistToken(ctx, token.TokenHash, token.ExpiresAt); err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresTokenRepository) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	if r.cache == nil {
		return false, errors.New("cache is nil")
	}
	return r.cache.SIsMember(ctx, tokenBlacklistKey, tokenHash)
}

func (r *PostgresTokenRepository) blacklistToken(ctx context.Context, tokenHash string, expiresAt time.Time) error {
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
		token.DeviceInfo = &deviceInfo.String
	}
	if ipAddress.Valid {
		token.IPAddress = &ipAddress.String
	}

	return &token, nil
}
