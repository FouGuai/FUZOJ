package gateway_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"fuzoj/internal/gateway/repository"
	"fuzoj/internal/gateway/service"
	pkgerrors "fuzoj/pkg/errors"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthServiceAuthenticate(t *testing.T) {
	secret := "test-secret"
	issuer := "fuzoj"
	setCache := newMockSetCache()
	banLocal := repository.NewLRUCache(32, time.Minute)
	banRepo := repository.NewBanCacheRepository(banLocal, setCache, time.Second, time.Minute)
	blacklistRepo := repository.NewTokenBlacklistRepository(nil, setCache, time.Second, time.Minute)
	authService := service.NewAuthService(secret, issuer, blacklistRepo, banRepo)

	accessToken := newAccessToken(t, secret, issuer, 123, "user")

	info, err := authService.Authenticate(context.Background(), accessToken)
	if err != nil {
		t.Fatalf("expected auth success, got error: %v", err)
	}
	if info.ID != 123 {
		t.Fatalf("unexpected user id: %d", info.ID)
	}
	if info.Role != "user" {
		t.Fatalf("unexpected role: %s", info.Role)
	}

	hash := hashToken(accessToken)
	_ = setCache.SAdd(context.Background(), "token:blacklist", hash)
	_, err = authService.Authenticate(context.Background(), accessToken)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.TokenInvalid {
		t.Fatalf("expected token invalid, got %v", err)
	}

	_ = setCache.SRem(context.Background(), "token:blacklist", hash)
	banRepo.MarkBanned(123, time.Minute)
	_, err = authService.Authenticate(context.Background(), accessToken)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.Forbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func newAccessToken(t *testing.T, secret, issuer string, userID int64, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"role": role,
		"typ":  "access",
		"sub":  fmt.Sprintf("%d", userID),
		"iss":  issuer,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}
	return raw
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
