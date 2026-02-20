package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	"fuzoj/internal/user/repository"
	pkgerrors "fuzoj/pkg/errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
	defaultLoginFailTTL    = 15 * time.Minute
	defaultLoginFailLimit  = 5
)

// AuthServiceConfig holds configuration for AuthService.
type AuthServiceConfig struct {
	JWTSecret       []byte
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	LoginFailTTL    time.Duration
	LoginFailLimit  int
}

// AuthService handles user authentication flows.
type AuthService struct {
	dbProvider     db.Provider
	users          repository.UserRepository
	tokens         repository.TokenRepository
	loginFailCache cache.BasicOps
	config         AuthServiceConfig
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	provider db.Provider,
	users repository.UserRepository,
	tokens repository.TokenRepository,
	loginFailCache cache.BasicOps,
	cfg AuthServiceConfig,
) *AuthService {
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

	return &AuthService{
		dbProvider:     provider,
		users:          users,
		tokens:         tokens,
		loginFailCache: loginFailCache,
		config:         cfg,
	}
}

// RegisterInput represents input for user registration.
type RegisterInput struct {
	Username string
	Password string
}

// LoginInput represents input for user login.
type LoginInput struct {
	Username   string
	Password   string
	IP         string
	DeviceInfo string
}

// RefreshInput represents input for token refresh.
type RefreshInput struct {
	RefreshToken string
}

// LogoutInput represents input for logout.
type LogoutInput struct {
	RefreshToken string
}

// UserInfo represents basic user info for auth responses.
type UserInfo struct {
	ID       int64
	Username string
	Role     repository.UserRole
}

// AuthResult represents the result of auth operations.
type AuthResult struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	User             UserInfo
}

// Register creates a new user and issues tokens.
func (s *AuthService) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	if err := validateUsername(input.Username); err != nil {
		return AuthResult{}, err
	}
	if err := validatePassword(input.Password); err != nil {
		return AuthResult{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
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
	err = s.withTransaction(ctx, func(tx db.Transaction) error {
		userID, createErr := s.users.Create(ctx, tx, user)
		if createErr != nil {
			return mapUserCreateError(createErr)
		}
		user.ID = userID

		resultData, tokenErr := s.issueTokens(ctx, tx, user, "", "")
		if tokenErr != nil {
			return tokenErr
		}
		result = resultData
		return nil
	})
	if err != nil {
		return AuthResult{}, err
	}
	return result, nil
}

// Login verifies credentials and issues tokens.
func (s *AuthService) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	if err := validateUsername(input.Username); err != nil {
		return AuthResult{}, err
	}
	if err := validateLoginPassword(input.Password); err != nil {
		return AuthResult{}, err
	}

	if err := s.checkLoginLimit(ctx, input.Username, input.IP); err != nil {
		return AuthResult{}, err
	}

	user, err := s.getUserByUsername(ctx, input.Username)
	if err != nil {
		if stderrors.Is(err, repository.ErrUserNotFound) {
			s.recordLoginFailure(ctx, input.Username, input.IP)
			return AuthResult{}, pkgerrors.New(pkgerrors.InvalidCredentials)
		}
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("get user failed: %w", err), pkgerrors.DatabaseError)
	}

	switch user.Status {
	case repository.UserStatusBanned:
		return AuthResult{}, pkgerrors.New(pkgerrors.AccountSuspended)
	case repository.UserStatusPendingVerify:
		return AuthResult{}, pkgerrors.New(pkgerrors.AccountNotActivated)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		s.recordLoginFailure(ctx, input.Username, input.IP)
		return AuthResult{}, pkgerrors.New(pkgerrors.InvalidCredentials)
	}

	s.clearLoginFailure(ctx, input.Username, input.IP)

	var result AuthResult
	err = s.withTransaction(ctx, func(tx db.Transaction) error {
		tokenResult, tokenErr := s.issueTokens(ctx, tx, user, input.DeviceInfo, input.IP)
		if tokenErr != nil {
			return tokenErr
		}
		result = tokenResult
		return nil
	})
	if err != nil {
		return AuthResult{}, err
	}

	return result, nil
}

// Refresh validates refresh token and issues new tokens.
func (s *AuthService) Refresh(ctx context.Context, input RefreshInput) (AuthResult, error) {
	claims, err := s.parseToken(input.RefreshToken, repository.TokenTypeRefresh)
	if err != nil {
		return AuthResult{}, err
	}
	userID, err := userIDFromClaims(claims)
	if err != nil {
		return AuthResult{}, err
	}
	hash := hashToken(input.RefreshToken)

	tokenRecord, err := s.tokens.GetByHash(ctx, nil, hash)
	if err != nil {
		if stderrors.Is(err, repository.ErrTokenNotFound) {
			return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
		}
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("get token failed: %w", err), pkgerrors.DatabaseError)
	}

	if tokenRecord.Revoked {
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if time.Now().After(tokenRecord.ExpiresAt) {
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenExpired)
	}
	if tokenRecord.TokenType != repository.TokenTypeRefresh {
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if tokenRecord.UserID != userID {
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}

	blacklisted, err := s.tokens.IsBlacklisted(ctx, hash)
	if err != nil {
		return AuthResult{}, pkgerrors.Wrap(fmt.Errorf("check token blacklist failed: %w", err), pkgerrors.CacheError)
	}
	if blacklisted {
		return AuthResult{}, pkgerrors.New(pkgerrors.TokenInvalid)
	}

	var result AuthResult
	err = s.withTransaction(ctx, func(tx db.Transaction) error {
		if err := s.tokens.RevokeByHash(ctx, tx, hash, tokenRecord.ExpiresAt); err != nil {
			if stderrors.Is(err, repository.ErrTokenNotFound) {
				return pkgerrors.New(pkgerrors.TokenInvalid)
			}
			return pkgerrors.Wrap(fmt.Errorf("revoke refresh token failed: %w", err), pkgerrors.DatabaseError)
		}

		user, err := s.getUserByID(ctx, tokenRecord.UserID)
		if err != nil {
			return err
		}

		switch user.Status {
		case repository.UserStatusBanned:
			return pkgerrors.New(pkgerrors.AccountSuspended)
		case repository.UserStatusPendingVerify:
			return pkgerrors.New(pkgerrors.AccountNotActivated)
		}

		deviceInfo := tokenRecord.DeviceInfo
		IPAddress := tokenRecord.IPAddress

		tokenResult, tokenErr := s.issueTokens(ctx, tx, user, deviceInfo, IPAddress)
		if tokenErr != nil {
			return tokenErr
		}
		result = tokenResult
		return nil
	})
	return result, err
}

// Logout revokes a refresh token.
func (s *AuthService) Logout(ctx context.Context, input LogoutInput) error {
	claims, err := s.parseToken(input.RefreshToken, repository.TokenTypeRefresh)
	if err != nil {
		return err
	}
	userID, err := userIDFromClaims(claims)
	if err != nil {
		return err
	}

	hash := hashToken(input.RefreshToken)
	tokenRecord, err := s.tokens.GetByHash(ctx, nil, hash)
	if err != nil {
		if stderrors.Is(err, repository.ErrTokenNotFound) {
			return pkgerrors.New(pkgerrors.TokenInvalid)
		}
		return pkgerrors.Wrap(fmt.Errorf("get token failed: %w", err), pkgerrors.DatabaseError)
	}

	if tokenRecord.TokenType != repository.TokenTypeRefresh {
		return pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if tokenRecord.UserID != userID {
		return pkgerrors.New(pkgerrors.TokenInvalid)
	}
	if tokenRecord.Revoked {
		return nil
	}

	if err := s.tokens.RevokeByHash(ctx, nil, hash, tokenRecord.ExpiresAt); err != nil {
		return pkgerrors.Wrap(fmt.Errorf("revoke refresh token failed: %w", err), pkgerrors.DatabaseError)
	}

	return nil
}

func (s *AuthService) withTransaction(ctx context.Context, fn func(tx db.Transaction) error) error {
	database, err := db.CurrentDatabase(s.dbProvider)
	if err != nil {
		return fn(nil)
	}
	if err := database.Transaction(ctx, fn); err != nil {
		if _, ok := err.(*pkgerrors.Error); ok {
			return err
		}
		return pkgerrors.Wrap(fmt.Errorf("transaction failed: %w", err), pkgerrors.TransactionFailed)
	}
	return nil
}

func (s *AuthService) issueTokens(ctx context.Context, tx db.Transaction, user *repository.User, deviceInfo, ip string) (AuthResult, error) {
	accessToken, accessExp, err := s.generateToken(user.ID, string(user.Role), repository.TokenTypeAccess, s.config.AccessTokenTTL)
	if err != nil {
		return AuthResult{}, err
	}
	refreshToken, refreshExp, err := s.generateToken(user.ID, string(user.Role), repository.TokenTypeRefresh, s.config.RefreshTokenTTL)
	if err != nil {
		return AuthResult{}, err
	}

	if err := s.tokens.Create(ctx, tx, &repository.UserToken{
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

	if err := s.tokens.Create(ctx, tx, &repository.UserToken{
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
			Role:     user.Role,
		},
	}, nil
}

func (s *AuthService) newTokenID() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", pkgerrors.Wrap(fmt.Errorf("generate token id failed: %w", err), pkgerrors.TokenGenerationFailed)
	}
	return hex.EncodeToString(randomBytes), nil
}

func (s *AuthService) getUserByID(ctx context.Context, userID int64) (*repository.User, error) {
	user, err := s.users.GetByID(ctx, nil, userID)
	if err != nil {
		if stderrors.Is(err, repository.ErrUserNotFound) {
			return nil, pkgerrors.New(pkgerrors.UserNotFound)
		}
		return nil, pkgerrors.Wrap(fmt.Errorf("get user failed: %w", err), pkgerrors.DatabaseError)
	}
	return user, nil
}

func (s *AuthService) getUserByUsername(ctx context.Context, username string) (*repository.User, error) {
	user, err := s.users.GetByUsername(ctx, nil, username)
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

// just for temporary place holder
func placeholderEmail(username string) string {
	return fmt.Sprintf("%s@local", username)
}
