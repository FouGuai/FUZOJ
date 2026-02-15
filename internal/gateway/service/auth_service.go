package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"fuzoj/internal/gateway/repository"
	pkgerrors "fuzoj/pkg/errors"

	"github.com/golang-jwt/jwt/v5"
)

type UserInfo struct {
	ID   int64
	Role string
}

type AuthService struct {
	jwtSecret []byte
	jwtIssuer string
	blacklist *repository.TokenBlacklistRepository
	banRepo   *repository.BanCacheRepository
}

func NewAuthService(jwtSecret, jwtIssuer string, blacklist *repository.TokenBlacklistRepository, banRepo *repository.BanCacheRepository) *AuthService {
	return &AuthService{
		jwtSecret: []byte(jwtSecret),
		jwtIssuer: jwtIssuer,
		blacklist: blacklist,
		banRepo:   banRepo,
	}
}

type tokenClaims struct {
	Role      string `json:"role"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

func (s *AuthService) Authenticate(ctx context.Context, raw string) (UserInfo, error) {
	if raw == "" {
		return UserInfo{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	claims, err := s.parseToken(raw)
	if err != nil {
		return UserInfo{}, err
	}
	userID, err := parseUserID(claims.Subject)
	if err != nil {
		return UserInfo{}, err
	}
	if s.blacklist != nil {
		hash := hashToken(raw)
		blacklisted, err := s.blacklist.IsBlacklisted(ctx, hash)
		if err != nil {
			return UserInfo{}, pkgerrors.Wrap(err, pkgerrors.ServiceUnavailable)
		}
		if blacklisted {
			return UserInfo{}, pkgerrors.New(pkgerrors.TokenInvalid)
		}
	}
	if s.banRepo != nil {
		banned, err := s.banRepo.IsBanned(ctx, userID)
		if err != nil {
			return UserInfo{}, pkgerrors.Wrap(err, pkgerrors.ServiceUnavailable)
		}
		if banned {
			return UserInfo{}, pkgerrors.New(pkgerrors.Forbidden).WithMessage("account suspended")
		}
	}
	return UserInfo{ID: userID, Role: claims.Role}, nil
}

func (s *AuthService) parseToken(raw string) (*tokenClaims, error) {
	if len(s.jwtSecret) == 0 {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	parsed, err := jwt.ParseWithClaims(raw, &tokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, pkgerrors.New(pkgerrors.TokenExpired)
		}
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if !parsed.Valid {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	claims, ok := parsed.Claims.(*tokenClaims)
	if !ok {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if s.jwtIssuer != "" && claims.Issuer != s.jwtIssuer {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if claims.TokenType != "access" {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if claims.Subject == "" {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	return claims, nil
}

func parseUserID(subject string) (int64, error) {
	userID, err := strconv.ParseInt(subject, 10, 64)
	if err != nil {
		return 0, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	return userID, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
