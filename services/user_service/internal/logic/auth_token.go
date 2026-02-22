package logic

import (
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"strconv"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/user_service/internal/repository"

	"github.com/golang-jwt/jwt/v5"
)

type tokenClaims struct {
	Role      string `json:"role"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

func (s *authManager) generateToken(userID int64, role string, tokenType repository.TokenType, ttl time.Duration) (string, time.Time, error) {
	if len(s.config.JWTSecret) == 0 {
		return "", time.Time{}, pkgerrors.New(pkgerrors.TokenGenerationFailed)
	}

	now := time.Now()
	expiresAt := now.Add(ttl)
	tokenID, err := s.newTokenID()
	if err != nil {
		return "", time.Time{}, err
	}
	claims := tokenClaims{
		Role:      role,
		TokenType: string(tokenType),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    s.config.JWTIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        tokenID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := token.SignedString(s.config.JWTSecret)
	if err != nil {
		return "", time.Time{}, pkgerrors.Wrap(fmt.Errorf("sign token failed: %w", err), pkgerrors.TokenGenerationFailed)
	}
	return raw, expiresAt, nil
}

func (s *authManager) parseToken(raw string, expectedType repository.TokenType) (*tokenClaims, error) {
	if raw == "" {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}

	parsed, err := jwt.ParseWithClaims(raw, &tokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.config.JWTSecret, nil
	})
	if err != nil {
		if stderrors.Is(err, jwt.ErrTokenExpired) {
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
	if s.config.JWTIssuer != "" && claims.Issuer != s.config.JWTIssuer {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if expectedType != "" && claims.TokenType != string(expectedType) {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if claims.Subject == "" {
		return nil, pkgerrors.New(pkgerrors.TokenInvalid)
	}

	return claims, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func userIDFromClaims(claims *tokenClaims) (int64, error) {
	if claims == nil || claims.Subject == "" {
		return 0, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	return userID, nil
}
