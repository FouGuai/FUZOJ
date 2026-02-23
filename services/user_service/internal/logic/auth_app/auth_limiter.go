package auth_app

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

func (s *authApp) checkLoginLimit(ctx context.Context, username, ip string) error {
	if s.loginFailRedis == nil {
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

func (s *authApp) recordLoginFailure(ctx context.Context, username, ip string) {
	if s.loginFailRedis == nil {
		return
	}

	s.incrementFailKey(ctx, loginFailUserPrefix+username)
	if ip != "" {
		s.incrementFailKey(ctx, loginFailIPPrefix+ip)
	}
}

func (s *authApp) clearLoginFailure(ctx context.Context, username, ip string) {
	if s.loginFailRedis == nil {
		return
	}

	keys := []string{loginFailUserPrefix + username}
	if ip != "" {
		keys = append(keys, loginFailIPPrefix+ip)
	}
	_, _ = s.loginFailRedis.DelCtx(ctx, keys...)
}

func (s *authApp) getFailCount(ctx context.Context, key string) int {
	value, err := s.loginFailRedis.GetCtx(ctx, key)
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

func (s *authApp) incrementFailKey(ctx context.Context, key string) {
	count, err := s.loginFailRedis.IncrCtx(ctx, key)
	if err != nil {
		logger.Warn(ctx, "increment login fail counter failed", zap.String("key", key), zap.Error(err))
		return
	}

	if count == 1 {
		ttlSeconds := int(s.config.LoginFailTTL.Seconds())
		if ttlSeconds > 0 {
			if err := s.loginFailRedis.ExpireCtx(ctx, key, ttlSeconds); err != nil {
				logger.Warn(ctx, "set login fail counter ttl failed", zap.String("key", key), zap.Error(err))
			}
		}
	}
}
