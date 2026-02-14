package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"fuzoj/internal/common/db"
	"fuzoj/internal/user/repository"
	"fuzoj/internal/user/service"
	pkgerrors "fuzoj/pkg/errors"

	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepo struct {
	usersByName map[string]*repository.User
	usersByID   map[int64]*repository.User
	nextID      int64
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		usersByName: make(map[string]*repository.User),
		usersByID:   make(map[int64]*repository.User),
		nextID:      1,
	}
}

func (r *fakeUserRepo) Create(ctx context.Context, tx db.Transaction, user *repository.User) (int64, error) {
	if user == nil {
		return 0, fmt.Errorf("user is nil")
	}
	if _, ok := r.usersByName[user.Username]; ok {
		return 0, repository.ErrUsernameExists
	}
	for _, existing := range r.usersByName {
		if existing.Email == user.Email {
			return 0, repository.ErrEmailExists
		}
	}
	id := r.nextID
	r.nextID++
	clone := *user
	clone.ID = id
	r.usersByName[user.Username] = &clone
	r.usersByID[id] = &clone
	return id, nil
}

func (r *fakeUserRepo) GetByID(ctx context.Context, tx db.Transaction, id int64) (*repository.User, error) {
	user, ok := r.usersByID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	clone := *user
	return &clone, nil
}

func (r *fakeUserRepo) GetByUsername(ctx context.Context, tx db.Transaction, username string) (*repository.User, error) {
	user, ok := r.usersByName[username]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	clone := *user
	return &clone, nil
}

func (r *fakeUserRepo) GetByEmail(ctx context.Context, tx db.Transaction, email string) (*repository.User, error) {
	for _, user := range r.usersByName {
		if user.Email == email {
			clone := *user
			return &clone, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func (r *fakeUserRepo) ExistsByUsername(ctx context.Context, tx db.Transaction, username string) (bool, error) {
	_, ok := r.usersByName[username]
	return ok, nil
}

func (r *fakeUserRepo) ExistsByEmail(ctx context.Context, tx db.Transaction, email string) (bool, error) {
	for _, user := range r.usersByName {
		if user.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeUserRepo) UpdatePassword(ctx context.Context, tx db.Transaction, userID int64, newHash string) error {
	user, ok := r.usersByID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	user.PasswordHash = newHash
	return nil
}

func (r *fakeUserRepo) UpdateStatus(ctx context.Context, tx db.Transaction, userID int64, status repository.UserStatus) error {
	user, ok := r.usersByID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	user.Status = status
	return nil
}

type fakeTokenRepo struct {
	tokens      map[string]*repository.UserToken
	blacklisted map[string]bool
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{
		tokens:      make(map[string]*repository.UserToken),
		blacklisted: make(map[string]bool),
	}
}

func (r *fakeTokenRepo) Create(ctx context.Context, tx db.Transaction, token *repository.UserToken) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	clone := *token
	r.tokens[token.TokenHash] = &clone
	return nil
}

func (r *fakeTokenRepo) GetByHash(ctx context.Context, tx db.Transaction, tokenHash string) (*repository.UserToken, error) {
	token, ok := r.tokens[tokenHash]
	if !ok {
		return nil, repository.ErrTokenNotFound
	}
	clone := *token
	return &clone, nil
}

func (r *fakeTokenRepo) RevokeByHash(ctx context.Context, tx db.Transaction, tokenHash string, expiresAt time.Time) error {
	token, ok := r.tokens[tokenHash]
	if !ok {
		return repository.ErrTokenNotFound
	}
	token.Revoked = true
	r.blacklisted[tokenHash] = true
	return nil
}

func (r *fakeTokenRepo) RevokeByUser(ctx context.Context, tx db.Transaction, userID int64) error {
	for hash, token := range r.tokens {
		if token.UserID == userID {
			token.Revoked = true
			r.blacklisted[hash] = true
		}
	}
	return nil
}

func (r *fakeTokenRepo) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	return r.blacklisted[tokenHash], nil
}

type fakeCache struct {
	values map[string]string
}

func newFakeCache() *fakeCache {
	return &fakeCache{values: make(map[string]string)}
}

func (c *fakeCache) Get(ctx context.Context, key string) (string, error) {
	return c.values[key], nil
}

func (c *fakeCache) MGet(ctx context.Context, keys ...string) ([]string, error) {
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, c.values[key])
	}
	return values, nil
}

func (c *fakeCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.values[key] = fmt.Sprint(value)
	return nil
}

func (c *fakeCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if _, ok := c.values[key]; ok {
		return false, nil
	}
	c.values[key] = fmt.Sprint(value)
	return true, nil
}

func (c *fakeCache) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	old := c.values[key]
	c.values[key] = fmt.Sprint(value)
	return old, nil
}

func (c *fakeCache) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(c.values, key)
	}
	return nil
}

func (c *fakeCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	var count int64
	for _, key := range keys {
		if _, ok := c.values[key]; ok {
			count++
		}
	}
	return count, nil
}

func (c *fakeCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (c *fakeCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return -1, nil
}

func (c *fakeCache) Incr(ctx context.Context, key string) (int64, error) {
	value := c.values[key]
	count, _ := strconv.ParseInt(value, 10, 64)
	count++
	c.values[key] = strconv.FormatInt(count, 10)
	return count, nil
}

func (c *fakeCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	current := c.values[key]
	count, _ := strconv.ParseInt(current, 10, 64)
	count += value
	c.values[key] = strconv.FormatInt(count, 10)
	return count, nil
}

func (c *fakeCache) Decr(ctx context.Context, key string) (int64, error) {
	current := c.values[key]
	count, _ := strconv.ParseInt(current, 10, 64)
	count--
	c.values[key] = strconv.FormatInt(count, 10)
	return count, nil
}

func (c *fakeCache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	current := c.values[key]
	count, _ := strconv.ParseInt(current, 10, 64)
	count -= value
	c.values[key] = strconv.FormatInt(count, 10)
	return count, nil
}

func newAuthServiceWithFakes(userRepo *fakeUserRepo, tokenRepo *fakeTokenRepo, cache *fakeCache) *service.AuthService {
	cfg := service.AuthServiceConfig{
		JWTSecret:       []byte("test-secret"),
		JWTIssuer:       "fuzoj",
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: time.Hour,
		LoginFailTTL:    time.Minute * 15,
		LoginFailLimit:  5,
	}

	return service.NewAuthService(nil, userRepo, tokenRepo, cache, cfg)
}

func newAuthServiceWithConfig(userRepo *fakeUserRepo, tokenRepo *fakeTokenRepo, cache *fakeCache, cfg service.AuthServiceConfig) *service.AuthService {
	return service.NewAuthService(nil, userRepo, tokenRepo, cache, cfg)
}

func TestAuthService_Register(t *testing.T) {
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeTokenRepo()
	cache := newFakeCache()
	authService := newAuthServiceWithFakes(userRepo, tokenRepo, cache)

	result, err := authService.Register(context.Background(), service.RegisterInput{
		Username: "alice",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatalf("tokens should not be empty")
	}
	if result.User.Username != "alice" {
		t.Fatalf("unexpected username: %s", result.User.Username)
	}

	_, err = authService.Register(context.Background(), service.RegisterInput{
		Username: "alice",
		Password: "password123",
	})
	if err == nil || !pkgerrors.Is(err, pkgerrors.UsernameAlreadyExists) {
		t.Fatalf("expected UsernameAlreadyExists, got %v", err)
	}
}

func TestAuthService_RegisterValidation(t *testing.T) {
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeTokenRepo()
	cache := newFakeCache()
	authService := newAuthServiceWithFakes(userRepo, tokenRepo, cache)

	tests := []struct {
		name     string
		username string
		password string
		errCode  pkgerrors.ErrorCode
	}{
		{
			name:     "invalid username",
			username: "ab",
			password: "password123",
			errCode:  pkgerrors.InvalidUsername,
		},
		{
			name:     "weak password",
			username: "valid_user",
			password: "short",
			errCode:  pkgerrors.PasswordTooWeak,
		},
		{
			name:     "password too long",
			username: "valid_user",
			password: strings.Repeat("a", 129),
			errCode:  pkgerrors.InvalidPassword,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := authService.Register(context.Background(), service.RegisterInput{
				Username: tc.username,
				Password: tc.password,
			})
			if err == nil || !pkgerrors.Is(err, tc.errCode) {
				t.Fatalf("expected %v, got %v", tc.errCode, err)
			}
		})
	}
}

func TestAuthService_LoginAndRateLimit(t *testing.T) {
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeTokenRepo()
	cache := newFakeCache()
	authService := newAuthServiceWithFakes(userRepo, tokenRepo, cache)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	_, _ = userRepo.Create(context.Background(), nil, &repository.User{
		Username:     "bob",
		Email:        "bob@local",
		PasswordHash: string(hash),
		Role:         repository.UserRoleUser,
		Status:       repository.UserStatusActive,
	})

	_, err := authService.Login(context.Background(), service.LoginInput{
		Username: "bob",
		Password: "password123",
		IP:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, err = authService.Login(context.Background(), service.LoginInput{
			Username: "bob",
			Password: "wrongpass",
			IP:       "127.0.0.1",
		})
		if err == nil || !pkgerrors.Is(err, pkgerrors.InvalidCredentials) {
			t.Fatalf("expected InvalidCredentials at attempt %d, got %v", i+1, err)
		}
	}

	_, err = authService.Login(context.Background(), service.LoginInput{
		Username: "bob",
		Password: "wrongpass",
		IP:       "127.0.0.1",
	})
	if err == nil || !pkgerrors.Is(err, pkgerrors.TooManyRequests) {
		t.Fatalf("expected TooManyRequests, got %v", err)
	}
}

func TestAuthService_LoginStatus(t *testing.T) {
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeTokenRepo()
	cache := newFakeCache()
	authService := newAuthServiceWithFakes(userRepo, tokenRepo, cache)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	_, _ = userRepo.Create(context.Background(), nil, &repository.User{
		Username:     "banned_user",
		Email:        "banned@local",
		PasswordHash: string(hash),
		Role:         repository.UserRoleUser,
		Status:       repository.UserStatusBanned,
	})
	_, _ = userRepo.Create(context.Background(), nil, &repository.User{
		Username:     "pending_user",
		Email:        "pending@local",
		PasswordHash: string(hash),
		Role:         repository.UserRoleUser,
		Status:       repository.UserStatusPendingVerify,
	})

	tests := []struct {
		name     string
		username string
		errCode  pkgerrors.ErrorCode
	}{
		{
			name:     "banned user",
			username: "banned_user",
			errCode:  pkgerrors.AccountSuspended,
		},
		{
			name:     "pending verify user",
			username: "pending_user",
			errCode:  pkgerrors.AccountNotActivated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := authService.Login(context.Background(), service.LoginInput{
				Username: tc.username,
				Password: "password123",
				IP:       "127.0.0.1",
			})
			if err == nil || !pkgerrors.Is(err, tc.errCode) {
				t.Fatalf("expected %v, got %v", tc.errCode, err)
			}
		})
	}
}

func TestAuthService_RefreshAndLogout(t *testing.T) {
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeTokenRepo()
	cache := newFakeCache()
	authService := newAuthServiceWithFakes(userRepo, tokenRepo, cache)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	_, _ = userRepo.Create(context.Background(), nil, &repository.User{
		Username:     "carol",
		Email:        "carol@local",
		PasswordHash: string(hash),
		Role:         repository.UserRoleUser,
		Status:       repository.UserStatusActive,
	})

	loginResult, err := authService.Login(context.Background(), service.LoginInput{
		Username: "carol",
		Password: "password123",
		IP:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	refreshResult, err := authService.Refresh(context.Background(), service.RefreshInput{
		RefreshToken: loginResult.RefreshToken,
	})
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if refreshResult.RefreshToken == loginResult.RefreshToken {
		t.Fatalf("refresh token should rotate, %s eq %s", refreshResult.RefreshToken, loginResult.RefreshToken)
	}

	oldHash := hashTokenForTest(loginResult.RefreshToken)
	if !tokenRepo.blacklisted[oldHash] {
		t.Fatalf("old refresh token should be blacklisted")
	}

	if err := authService.Logout(context.Background(), service.LogoutInput{
		RefreshToken: refreshResult.RefreshToken,
	}); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	newHash := hashTokenForTest(refreshResult.RefreshToken)
	if !tokenRepo.blacklisted[newHash] {
		t.Fatalf("refresh token should be blacklisted after logout")
	}
}

func TestAuthService_RefreshInvalid(t *testing.T) {
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeTokenRepo()
	cache := newFakeCache()
	authService := newAuthServiceWithFakes(userRepo, tokenRepo, cache)

	if _, err := authService.Refresh(context.Background(), service.RefreshInput{
		RefreshToken: "",
	}); err == nil || !pkgerrors.Is(err, pkgerrors.TokenInvalid) {
		t.Fatalf("expected TokenInvalid, got %v", err)
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	_, _ = userRepo.Create(context.Background(), nil, &repository.User{
		Username:     "dave",
		Email:        "dave@local",
		PasswordHash: string(hash),
		Role:         repository.UserRoleUser,
		Status:       repository.UserStatusActive,
	})
	loginResult, err := authService.Login(context.Background(), service.LoginInput{
		Username: "dave",
		Password: "password123",
		IP:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	refreshHash := hashTokenForTest(loginResult.RefreshToken)
	tokenRepo.blacklisted[refreshHash] = true

	if _, err := authService.Refresh(context.Background(), service.RefreshInput{
		RefreshToken: loginResult.RefreshToken,
	}); err == nil || !pkgerrors.Is(err, pkgerrors.TokenInvalid) {
		t.Fatalf("expected TokenInvalid, got %v", err)
	}

	expiredAuthService := newAuthServiceWithConfig(userRepo, tokenRepo, cache, service.AuthServiceConfig{
		JWTSecret:       []byte("test-secret"),
		JWTIssuer:       "fuzoj",
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: -time.Minute,
		LoginFailTTL:    time.Minute * 15,
		LoginFailLimit:  5,
	})

	expiredLogin, err := expiredAuthService.Login(context.Background(), service.LoginInput{
		Username: "dave",
		Password: "password123",
		IP:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if _, err := expiredAuthService.Refresh(context.Background(), service.RefreshInput{
		RefreshToken: expiredLogin.RefreshToken,
	}); err == nil || !pkgerrors.Is(err, pkgerrors.TokenExpired) {
		t.Fatalf("expected TokenExpired, got %v", err)
	}
}

func hashTokenForTest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
