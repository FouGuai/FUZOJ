package gateway_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"fuzoj/pkg/errors"
	"fuzoj/services/gateway_service/internal/repository"
	"fuzoj/services/gateway_service/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestAuthServiceAuthenticate(t *testing.T) {
	secret := "test-secret"
	issuer := "fuzoj"
	mini := miniredis.RunT(t)
	redisClient, err := redis.NewRedis(redis.RedisConf{Host: mini.Addr(), Type: "node"})
	if err != nil {
		t.Fatalf("init redis failed: %v", err)
	}

	banLocal := repository.NewLRUCache(32, time.Minute)
	banRepo := repository.NewBanCacheRepository(banLocal, redisClient, time.Minute)
	blacklistRepo := repository.NewTokenBlacklistRepository(nil, redisClient, time.Minute)
	authService := service.NewAuthService(secret, issuer, blacklistRepo, banRepo)

	accessToken := newAccessToken(t, secret, issuer, 123, "user", 5*time.Minute)

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
	mini.SAdd("token:blacklist", hash)
	_, err = authService.Authenticate(context.Background(), accessToken)
	if err == nil || errors.GetCode(err) != errors.TokenInvalid {
		t.Fatalf("expected token invalid, got %v", err)
	}

	mini.SRem("token:blacklist", hash)
	banRepo.MarkBanned(123, time.Minute)
	_, err = authService.Authenticate(context.Background(), accessToken)
	if err == nil || errors.GetCode(err) != errors.Forbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
