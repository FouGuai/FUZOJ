package user_service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"fuzoj/services/user_service/internal/config"
	"fuzoj/services/user_service/internal/repository"
	"fuzoj/services/user_service/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepo struct {
	mu         sync.Mutex
	nextID     int64
	byID       map[int64]*repository.User
	byUsername map[string]*repository.User
	byEmail    map[string]*repository.User
	lastCreate *repository.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		nextID:     1,
		byID:       make(map[int64]*repository.User),
		byUsername: make(map[string]*repository.User),
		byEmail:    make(map[string]*repository.User),
	}
}

func (r *fakeUserRepo) WithSession(session sqlx.Session) repository.UserRepository {
	return r
}

func (r *fakeUserRepo) Create(ctx context.Context, user *repository.User) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if user == nil {
		return 0, fmt.Errorf("user is nil")
	}
	if _, ok := r.byUsername[user.Username]; ok {
		return 0, repository.ErrUsernameExists
	}
	if _, ok := r.byEmail[user.Email]; ok {
		return 0, repository.ErrEmailExists
	}
	userCopy := *user
	userCopy.ID = r.nextID
	r.nextID++
	r.byID[userCopy.ID] = &userCopy
	r.byUsername[userCopy.Username] = &userCopy
	r.byEmail[userCopy.Email] = &userCopy
	r.lastCreate = &userCopy
	return userCopy.ID, nil
}

func (r *fakeUserRepo) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

func (r *fakeUserRepo) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byUsername[username]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

func (r *fakeUserRepo) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byEmail[email]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

func (r *fakeUserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.byUsername[username]
	return ok, nil
}

func (r *fakeUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.byEmail[email]
	return ok, nil
}

func (r *fakeUserRepo) UpdatePassword(ctx context.Context, userID int64, newHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	user.PasswordHash = newHash
	return nil
}

func (r *fakeUserRepo) UpdateStatus(ctx context.Context, userID int64, status repository.UserStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	user.Status = status
	return nil
}

func (r *fakeUserRepo) UpdateRole(ctx context.Context, userID int64, role repository.UserRole) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	user.Role = role
	return nil
}

func (r *fakeUserRepo) UpdateEmail(ctx context.Context, userID int64, email string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.byID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	if existing, ok := r.byEmail[email]; ok && existing.ID != userID {
		return repository.ErrEmailExists
	}
	delete(r.byEmail, user.Email)
	user.Email = email
	r.byEmail[email] = user
	return nil
}

type fakeTokenRepo struct {
	mu          sync.Mutex
	byHash      map[string]*repository.UserToken
	blacklisted map[string]struct{}
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{
		byHash:      make(map[string]*repository.UserToken),
		blacklisted: make(map[string]struct{}),
	}
}

func (r *fakeTokenRepo) WithSession(session sqlx.Session) repository.TokenRepository {
	return r
}

func (r *fakeTokenRepo) Create(ctx context.Context, token *repository.UserToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	copy := *token
	r.byHash[token.TokenHash] = &copy
	return nil
}

func (r *fakeTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.UserToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.byHash[tokenHash]
	if !ok {
		return nil, repository.ErrTokenNotFound
	}
	copy := *token
	return &copy, nil
}

func (r *fakeTokenRepo) RevokeByHash(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.byHash[tokenHash]
	if !ok {
		return repository.ErrTokenNotFound
	}
	token.Revoked = true
	r.blacklisted[tokenHash] = struct{}{}
	return nil
}

func (r *fakeTokenRepo) RevokeByUser(ctx context.Context, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for hash, token := range r.byHash {
		if token.UserID == userID && !token.Revoked {
			token.Revoked = true
			r.blacklisted[hash] = struct{}{}
		}
	}
	return nil
}

func (r *fakeTokenRepo) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.blacklisted[tokenHash]
	return ok, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

type testDeps struct {
	users  *fakeUserRepo
	tokens *fakeTokenRepo
	svcCtx *svc.ServiceContext
}

func newTestDeps() *testDeps {
	users := newFakeUserRepo()
	tokens := newFakeTokenRepo()
	cfg := config.Config{
		Auth: config.AuthConfig{
			JWTSecret:       "test-secret",
			JWTIssuer:       "fuzoj-test",
			AccessTokenTTL:  time.Minute,
			RefreshTokenTTL: time.Hour,
			LoginFailTTL:    time.Minute,
			LoginFailLimit:  5,
			Root:            config.RootAccountConfig{Enabled: false},
		},
	}
	svcCtx := &svc.ServiceContext{
		Config:    cfg,
		Conn:      nil,
		Redis:     nil,
		UserRepo:  users,
		TokenRepo: tokens,
	}
	return &testDeps{
		users:  users,
		tokens: tokens,
		svcCtx: svcCtx,
	}
}

func mustHashPassword(t *testing.T, raw string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	return string(hash)
}
