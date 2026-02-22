package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"time"

	commoncache "fuzoj/internal/common/cache"
	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"
	"fuzoj/services/user_service/internal/config"
	"fuzoj/services/user_service/internal/repository"
	"fuzoj/services/user_service/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
	defaultLoginFailTTL    = 15 * time.Minute
	defaultLoginFailLimit  = 5
	rootInitTimeout        = 5 * time.Second
)

type authConfig struct {
	JWTSecret       []byte
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	LoginFailTTL    time.Duration
	LoginFailLimit  int
	Root            config.RootAccountConfig
}

type authManager struct {
	conn           sqlx.SqlConn
	users          repository.UserRepository
	tokens         repository.TokenRepository
	loginFailCache commoncache.BasicOps
	config         authConfig
}

func newAuthManager(svcCtx *svc.ServiceContext) *authManager {
	cfg := authConfig{}
	if svcCtx != nil {
		cfg = authConfig{
			JWTSecret:       []byte(svcCtx.Config.Auth.JWTSecret),
			JWTIssuer:       svcCtx.Config.Auth.JWTIssuer,
			AccessTokenTTL:  svcCtx.Config.Auth.AccessTokenTTL,
			RefreshTokenTTL: svcCtx.Config.Auth.RefreshTokenTTL,
			LoginFailTTL:    svcCtx.Config.Auth.LoginFailTTL,
			LoginFailLimit:  svcCtx.Config.Auth.LoginFailLimit,
			Root:            svcCtx.Config.Auth.Root,
		}
	}
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = defaultAccessTokenTTL
	}
	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = defaultRefreshTokenTTL
	}
	if cfg.LoginFailTTL == 0 {
		cfg.LoginFailTTL = defaultLoginFailTTL
	}
	if cfg.LoginFailLimit == 0 {
		cfg.LoginFailLimit = defaultLoginFailLimit
	}
	if cfg.JWTIssuer == "" {
		cfg.JWTIssuer = "fuzoj"
	}

	if svcCtx == nil {
		return &authManager{config: cfg}
	}

	return &authManager{
		conn:           svcCtx.Conn,
		users:          svcCtx.UserRepo,
		tokens:         svcCtx.TokenRepo,
		loginFailCache: svcCtx.LoginFailCache,
		config:         cfg,
	}
}

func InitAuth(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx == nil {
		return
	}
	manager := newAuthManager(svcCtx)
	manager.ensureRootAccount(ctx)
}

func (s *authManager) ensureRootAccount(parent context.Context) {
	cfg := s.config.Root
	if !cfg.Enabled {
		return
	}

	ctx, cancel := context.WithTimeout(parent, rootInitTimeout)
	defer cancel()

	logger.Info(ctx, "root account init start", zap.String("username", cfg.Username))

	if err := validateUsername(cfg.Username); err != nil {
		logger.Error(ctx, "root account invalid username", zap.String("username", cfg.Username), zap.Error(err))
		return
	}
	if err := validatePassword(cfg.Password); err != nil {
		logger.Error(ctx, "root account invalid password", zap.String("username", cfg.Username), zap.Error(err))
		return
	}

	existing, err := s.users.GetByUsername(ctx, cfg.Username)
	if err == nil {
		logger.Info(ctx, "root account already exists", zap.Int64("user_id", existing.ID), zap.String("username", existing.Username))
		return
	}
	if !stderrors.Is(err, repository.ErrUserNotFound) {
		logger.Error(ctx, "root account lookup failed", zap.String("username", cfg.Username), zap.Error(err))
		return
	}

	result, err := s.Register(ctx, RegisterInput{
		Username: cfg.Username,
		Password: cfg.Password,
	})
	if err != nil {
		if pkgerrors.Is(err, pkgerrors.UsernameAlreadyExists) {
			logger.Info(ctx, "root account already exists", zap.String("username", cfg.Username))
			return
		}
		logger.Error(ctx, "root account register failed", zap.String("username", cfg.Username), zap.Error(err))
		return
	}

	userID := result.User.ID
	err = s.withTransaction(ctx, func(session sqlx.Session) error {
		usersRepo := s.users.WithSession(session)
		if err := usersRepo.UpdateRole(ctx, userID, repository.UserRoleSuperAdmin); err != nil {
			return err
		}
		if cfg.Email != "" {
			if err := usersRepo.UpdateEmail(ctx, userID, cfg.Email); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if stderrors.Is(err, repository.ErrEmailExists) {
			logger.Warn(ctx, "root account email already exists", zap.Int64("user_id", userID), zap.String("email", cfg.Email))
			logger.Info(ctx, "root account created with default email", zap.Int64("user_id", userID), zap.String("username", cfg.Username))
			return
		}
		logger.Error(ctx, "root account update failed", zap.Int64("user_id", userID), zap.String("username", cfg.Username), zap.Error(err))
		return
	}
	logger.Info(ctx, "root account created", zap.Int64("user_id", userID), zap.String("username", cfg.Username))
}

func (s *authManager) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	logger.Info(ctx, "auth register start", zap.String("username", input.Username))
	if err := validateUsername(input.Username); err != nil {
		logger.Warn(ctx, "auth register invalid username", zap.String("username", input.Username), zap.Error(err))
		return AuthResult{}, err
	}
	if err := validatePassword(input.Password); err != nil {
		logger.Warn(ctx, "auth register invalid password", zap.String("username", input.Username), zap.Error(err))
		return AuthResult{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "auth register hash password failed", zap.String("username", input.Username), zap.Error(err))
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("hash password failed: %w", err), pkgerrors.InternalServerError)
	}

	user := &repository.User{
		Username:     input.Username,
		Email:        placeholderEmail(input.Username),
		PasswordHash: string(passwordHash),
		Role:         repository.UserRoleUser,
		Status:       repository.UserStatusActive,
	}

	var result AuthResult
	err = s.withTransaction(ctx, func(session sqlx.Session) error {
		usersRepo := s.users.WithSession(session)
		tokensRepo := s.tokens.WithSession(session)
		userID, createErr := usersRepo.Create(ctx, user)
		if createErr != nil {
			logger.Warn(ctx, "auth register create user failed", zap.String("username", input.Username), zap.Error(createErr))
			return mapUserCreateError(createErr)
		}
		user.ID = userID

		resultData, tokenErr := s.issueTokens(ctx, tokensRepo, user, "", "")
		if tokenErr != nil {
			logger.Warn(ctx, "auth register issue tokens failed", zap.Int64("user_id", userID), zap.Error(tokenErr))
			return tokenErr
		}
		result = resultData
		return nil
	})
	if err != nil {
		logger.Warn(ctx, "auth register failed", zap.String("username", input.Username), zap.Error(err))
		return AuthResult{}, err
	}
	logger.Info(ctx, "auth register success", zap.Int64("user_id", user.ID), zap.String("username", input.Username))
	return result, nil
}

func (s *authManager) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	logger.Info(ctx, "auth login start", zap.String("username", input.Username), zap.String("ip", input.IP))

	if err := s.checkLoginLimit(ctx, input.Username, input.IP); err != nil {
		logger.Warn(ctx, "auth login blocked by rate limit", zap.String("username", input.Username), zap.String("ip", input.IP), zap.Error(err))
		return AuthResult{}, err
	}

	user, err := s.getUserByUsername(ctx, s.users, input.Username)
	if err != nil {
		if stderrors.Is(err, repository.ErrUserNotFound) {
			s.recordLoginFailure(ctx, input.Username, input.IP)
			logger.Warn(ctx, "auth login user not found", zap.String("username", input.Username), zap.String("ip", input.IP))
			return AuthResult{}, pkgerrors.New(pkgerrors.InvalidCredentials)
		}
		logger.Error(ctx, "auth login get user failed", zap.String("username", input.Username), zap.String("ip", input.IP), zap.Error(err))
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("get user failed: %w", err), pkgerrors.DatabaseError)
	}

	switch user.Status {
	case repository.UserStatusBanned:
		logger.Warn(ctx, "auth login blocked by banned status", zap.Int64("user_id", user.ID), zap.String("username", input.Username))
		return AuthResult{}, pkgerrors.New(pkgerrors.AccountSuspended)
	case repository.UserStatusPendingVerify:
		logger.Warn(ctx, "auth login blocked by pending verification", zap.Int64("user_id", user.ID), zap.String("username", input.Username))
		return AuthResult{}, pkgerrors.New(pkgerrors.AccountNotActivated)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		s.recordLoginFailure(ctx, input.Username, input.IP)
		logger.Warn(ctx, "auth login invalid credentials", zap.Int64("user_id", user.ID), zap.String("username", input.Username), zap.String("ip", input.IP))
		return AuthResult{}, pkgerrors.New(pkgerrors.InvalidCredentials)
	}

	s.clearLoginFailure(ctx, input.Username, input.IP)

	var result AuthResult
	err = s.withTransaction(ctx, func(session sqlx.Session) error {
		tokensRepo := s.tokens.WithSession(session)
		tokenResult, tokenErr := s.issueTokens(ctx, tokensRepo, user, input.DeviceInfo, input.IP)
		if tokenErr != nil {
			logger.Warn(ctx, "auth login issue tokens failed", zap.Int64("user_id", user.ID), zap.String("username", input.Username), zap.Error(tokenErr))
			return tokenErr
		}
		result = tokenResult
		return nil
	})
	if err != nil {
		logger.Warn(ctx, "auth login failed", zap.Int64("user_id", user.ID), zap.String("username", input.Username), zap.Error(err))
		return AuthResult{}, err
	}

	logger.Info(ctx, "auth login success", zap.Int64("user_id", user.ID), zap.String("username", input.Username))
	return result, nil
}

func (s *authManager) Refresh(ctx context.Context, input RefreshInput) (AuthResult, error) {
	logger.Info(ctx, "auth refresh start")
	claims, err := s.parseToken(input.RefreshToken, repository.TokenTypeRefresh)
	if err != nil {
		logger.Warn(ctx, "auth refresh parse token failed", zap.Error(err))
		return AuthResult{}, err
	}
	userID, err := userIDFromClaims(claims)
	if err != nil {
		logger.Warn(ctx, "auth refresh invalid claims", zap.Error(err))
		return AuthResult{}, err
	}
	hash := hashToken(input.RefreshToken)

	tokenRecord, err := s.tokens.GetByHash(ctx, hash)
	if err != nil {
		if stderrors.Is(err, repository.ErrTokenNotFound) {
			logger.Warn(ctx, "auth refresh token not found", zap.Int64("user_id", userID))
			return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
		}
		logger.Error(ctx, "auth refresh get token failed", zap.Int64("user_id", userID), zap.Error(err))
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("get token failed: %w", err), pkgerrors.DatabaseError)
	}

	if tokenRecord.Revoked {
		logger.Warn(ctx, "auth refresh token revoked", zap.Int64("user_id", userID))
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if time.Now().After(tokenRecord.ExpiresAt) {
		logger.Warn(ctx, "auth refresh token expired", zap.Int64("user_id", userID))
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenExpired)
	}
	if tokenRecord.TokenType != repository.TokenTypeRefresh {
		logger.Warn(ctx, "auth refresh token type mismatch", zap.Int64("user_id", userID))
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if tokenRecord.UserID != userID {
		logger.Warn(ctx, "auth refresh token user mismatch", zap.Int64("user_id", userID), zap.Int64("token_user_id", tokenRecord.UserID))
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}

	blacklisted, err := s.tokens.IsBlacklisted(ctx, hash)
	if err != nil {
		logger.Error(ctx, "auth refresh blacklist check failed", zap.Int64("user_id", userID), zap.Error(err))
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("check token blacklist failed: %w", err), pkgerrors.CacheError)
	}
	if blacklisted {
		logger.Warn(ctx, "auth refresh token blacklisted", zap.Int64("user_id", userID))
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}

	var result AuthResult
	err = s.withTransaction(ctx, func(session sqlx.Session) error {
		usersRepo := s.users.WithSession(session)
		tokensRepo := s.tokens.WithSession(session)
		if err := tokensRepo.RevokeByHash(ctx, hash, tokenRecord.ExpiresAt); err != nil {
			if stderrors.Is(err, repository.ErrTokenNotFound) {
				logger.Warn(ctx, "auth refresh token revoked by another flow", zap.Int64("user_id", userID))
				return pkgerrors.New(pkgerrors.TokenInvalid)
			}
			logger.Error(ctx, "auth refresh revoke token failed", zap.Int64("user_id", userID), zap.Error(err))
			return pkgerrors.Wrap(fmt.Errorf("revoke refresh token failed: %w", err), pkgerrors.DatabaseError)
		}

		user, err := s.getUserByID(ctx, usersRepo, tokenRecord.UserID)
		if err != nil {
			logger.Warn(ctx, "auth refresh get user failed", zap.Int64("user_id", userID), zap.Error(err))
			return err
		}

		switch user.Status {
		case repository.UserStatusBanned:
			logger.Warn(ctx, "auth refresh blocked by banned status", zap.Int64("user_id", user.ID))
			return pkgerrors.New(pkgerrors.AccountSuspended)
		case repository.UserStatusPendingVerify:
			logger.Warn(ctx, "auth refresh blocked by pending verification", zap.Int64("user_id", user.ID))
			return pkgerrors.New(pkgerrors.AccountNotActivated)
		}

		deviceInfo := tokenRecord.DeviceInfo
		ipAddress := tokenRecord.IPAddress

		tokenResult, tokenErr := s.issueTokens(ctx, tokensRepo, user, deviceInfo, ipAddress)
		if tokenErr != nil {
			logger.Warn(ctx, "auth refresh issue tokens failed", zap.Int64("user_id", user.ID), zap.Error(tokenErr))
			return tokenErr
		}
		result = tokenResult
		return nil
	})
	if err != nil {
		logger.Warn(ctx, "auth refresh failed", zap.Int64("user_id", userID), zap.Error(err))
		return AuthResult{}, err
	}
	logger.Info(ctx, "auth refresh success", zap.Int64("user_id", userID))
	return result, nil
}

func (s *authManager) Logout(ctx context.Context, input LogoutInput) error {
	logger.Info(ctx, "auth logout start")
	claims, err := s.parseToken(input.RefreshToken, repository.TokenTypeRefresh)
	if err != nil {
		logger.Warn(ctx, "auth logout parse token failed", zap.Error(err))
		return err
	}
	userID, err := userIDFromClaims(claims)
	if err != nil {
		logger.Warn(ctx, "auth logout invalid claims", zap.Error(err))
		return err
	}

	hash := hashToken(input.RefreshToken)
	tokenRecord, err := s.tokens.GetByHash(ctx, hash)
	if err != nil {
		if stderrors.Is(err, repository.ErrTokenNotFound) {
			logger.Warn(ctx, "auth logout token not found", zap.Int64("user_id", userID))
			return pkgerrors.New(pkgerrors.TokenInvalid)
		}
		logger.Error(ctx, "auth logout get token failed", zap.Int64("user_id", userID), zap.Error(err))
		return pkgerrors.Wrap(fmt.Errorf("get token failed: %w", err), pkgerrors.DatabaseError)
	}

	if tokenRecord.TokenType != repository.TokenTypeRefresh {
		logger.Warn(ctx, "auth logout token type mismatch", zap.Int64("user_id", userID))
		return pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if tokenRecord.UserID != userID {
		logger.Warn(ctx, "auth logout token user mismatch", zap.Int64("user_id", userID), zap.Int64("token_user_id", tokenRecord.UserID))
		return pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if tokenRecord.Revoked {
		logger.Info(ctx, "auth logout token already revoked", zap.Int64("user_id", userID))
		return nil
	}

	if err := s.tokens.RevokeByHash(ctx, hash, tokenRecord.ExpiresAt); err != nil {
		logger.Error(ctx, "auth logout revoke token failed", zap.Int64("user_id", userID), zap.Error(err))
		return pkgerrors.Wrap(fmt.Errorf("revoke refresh token failed: %w", err), pkgerrors.DatabaseError)
	}

	logger.Info(ctx, "auth logout success", zap.Int64("user_id", userID))
	return nil
}

func (s *authManager) withTransaction(ctx context.Context, fn func(session sqlx.Session) error) error {
	if s.conn == nil {
		return fn(nil)
	}
	if err := s.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return fn(session)
	}); err != nil {
		if _, ok := err.(*pkgerrors.Error); ok {
			return err
		}
		return pkgerrors.Wrap(fmt.Errorf("transaction failed: %w", err), pkgerrors.TransactionFailed)
	}
	return nil
}

func (s *authManager) issueTokens(ctx context.Context, tokens repository.TokenRepository, user *repository.User, deviceInfo, ip string) (AuthResult, error) {
	accessToken, accessExp, err := s.generateToken(user.ID, string(user.Role), repository.TokenTypeAccess, s.config.AccessTokenTTL)
	if err != nil {
		return AuthResult{}, err
	}
	refreshToken, refreshExp, err := s.generateToken(user.ID, string(user.Role), repository.TokenTypeRefresh, s.config.RefreshTokenTTL)
	if err != nil {
		return AuthResult{}, err
	}

	if err := tokens.Create(ctx, &repository.UserToken{
		UserID:     user.ID,
		TokenHash:  hashToken(accessToken),
		TokenType:  repository.TokenTypeAccess,
		ExpiresAt:  accessExp,
		Revoked:    false,
		DeviceInfo: deviceInfo,
		IPAddress:  ip,
	}); err != nil {
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("create access token record failed: %w", err), pkgerrors.DatabaseError)
	}

	if err := tokens.Create(ctx, &repository.UserToken{
		UserID:     user.ID,
		TokenHash:  hashToken(refreshToken),
		TokenType:  repository.TokenTypeRefresh,
		ExpiresAt:  refreshExp,
		Revoked:    false,
		DeviceInfo: deviceInfo,
		IPAddress:  ip,
	}); err != nil {
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("create refresh token record failed: %w", err), pkgerrors.DatabaseError)
	}

	return AuthResult{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Role:     string(user.Role),
		},
	}, nil
}

func (s *authManager) newTokenID() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", pkgerrors.Wrap(fmt.Errorf("generate token id failed: %w", err), pkgerrors.TokenGenerationFailed)
	}
	return hex.EncodeToString(randomBytes), nil
}

func (s *authManager) getUserByID(ctx context.Context, users repository.UserRepository, userID int64) (*repository.User, error) {
	user, err := users.GetByID(ctx, userID)
	if err != nil {
		if stderrors.Is(err, repository.ErrUserNotFound) {
			return nil, pkgerrors.New(pkgerrors.UserNotFound)
		}
		return nil, pkgerrors.Wrap(fmt.Errorf("get user failed: %w", err), pkgerrors.DatabaseError)
	}
	return user, nil
}

func (s *authManager) getUserByUsername(ctx context.Context, users repository.UserRepository, username string) (*repository.User, error) {
	user, err := users.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func mapUserCreateError(err error) error {
	if stderrors.Is(err, repository.ErrUsernameExists) {
		return pkgerrors.New(pkgerrors.UsernameAlreadyExists)
	}
	if stderrors.Is(err, repository.ErrEmailExists) {
		return pkgerrors.New(pkgerrors.EmailAlreadyExists)
	}
	if stderrors.Is(err, repository.ErrDuplicate) {
		return pkgerrors.New(pkgerrors.RecordAlreadyExists)
	}
	return pkgerrors.Wrap(fmt.Errorf("create user failed: %w", err), pkgerrors.DatabaseError)
}

func placeholderEmail(username string) string {
	return fmt.Sprintf("%s@local", username)
}
