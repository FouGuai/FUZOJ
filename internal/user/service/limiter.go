package service

import (
	"context"
	"strconv"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

const (
	loginFailUserPrefix = "login:fail:username:"
	loginFailIPPrefix   = "login:fail:ip:"
)

func (s *AuthService) checkLoginLimit(ctx context.Context, username, ip string) error {
	if s.loginFailCache == nil {
		return nil
	}
	if s.config.LoginFailLimit <= 0 {
		return nil
	}

	userCount := s.getFailCount(ctx, loginFailUserPrefix+username)
	ipCount := 0
	if ip != "" {
		ipCount = s.getFailCount(ctx, loginFailIPPrefix+ip)
	}

	if userCount >= s.config.LoginFailLimit || ipCount >= s.config.LoginFailLimit {
		return pkgerrors.New(pkgerrors.TooManyRequests)
	}
	return nil
}

func (s *AuthService) recordLoginFailure(ctx context.Context, username, ip string) {
	if s.loginFailCache == nil {
		return
	}

	s.incrementFailKey(ctx, loginFailUserPrefix+username)
	if ip != "" {
		s.incrementFailKey(ctx, loginFailIPPrefix+ip)
	}
}

func (s *AuthService) clearLoginFailure(ctx context.Context, username, ip string) {
	if s.loginFailCache == nil {
		return
	}

	keys := []string{loginFailUserPrefix + username}
	if ip != "" {
		keys = append(keys, loginFailIPPrefix+ip)
	}
	_ = s.loginFailCache.Del(ctx, keys...)
}

func (s *AuthService) getFailCount(ctx context.Context, key string) int {
	value, err := s.loginFailCache.Get(ctx, key)
	if err != nil {
		logger.Warn(ctx, "get login fail counter failed", zap.String("key", key), zap.Error(err))
		return 0
	}
	if value == "" {
		return 0
	}

	count, err := strconv.Atoi(value)
	if err != nil {
		logger.Warn(ctx, "parse login fail counter failed", zap.String("key", key), zap.Error(err))
		return 0
	}
	return count
}

func (s *AuthService) incrementFailKey(ctx context.Context, key string) {
	count, err := s.loginFailCache.Incr(ctx, key)
	if err != nil {
		logger.Warn(ctx, "increment login fail counter failed", zap.String("key", key), zap.Error(err))
		return
	}

	if count == 1 {
		if err := s.loginFailCache.Expire(ctx, key, s.config.LoginFailTTL); err != nil {
			logger.Warn(ctx, "set login fail counter ttl failed", zap.String("key", key), zap.Error(err))
		}
	}
}
