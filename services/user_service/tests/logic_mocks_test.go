package user_service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	commoncache "fuzoj/internal/common/cache"
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

type fakeBasicCache struct {
	mu      sync.Mutex
	values  map[string]string
	expires map[string]time.Time
}

func newFakeBasicCache() *fakeBasicCache {
	return &fakeBasicCache{
		values:  make(map[string]string),
		expires: make(map[string]time.Time),
	}
}

func (c *fakeBasicCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(key) {
		return "", nil
	}
	return c.values[key], nil
}

func (c *fakeBasicCache) MGet(ctx context.Context, keys ...string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	results := make([]string, 0, len(keys))
	for _, key := range keys {
		if c.isExpiredLocked(key) {
			results = append(results, "")
			continue
		}
		results = append(results, c.values[key])
	}
	return results, nil
}

func (c *fakeBasicCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = fmt.Sprint(value)
	c.setTTLLocked(key, ttl)
	return nil
}

func (c *fakeBasicCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.isExpiredLocked(key)
	if _, ok := c.values[key]; ok {
		return false, nil
	}
	c.values[key] = fmt.Sprint(value)
	c.setTTLLocked(key, ttl)
	return true, nil
}

func (c *fakeBasicCache) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev := ""
	c.isExpiredLocked(key)
	if val, ok := c.values[key]; ok {
		prev = val
	}
	c.values[key] = fmt.Sprint(value)
	delete(c.expires, key)
	return prev, nil
}

func (c *fakeBasicCache) Del(ctx context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range keys {
		delete(c.values, key)
		delete(c.expires, key)
	}
	return nil
}

func (c *fakeBasicCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var count int64
	for _, key := range keys {
		if c.isExpiredLocked(key) {
			continue
		}
		if _, ok := c.values[key]; ok {
			count++
		}
	}
	return count, nil
}

func (c *fakeBasicCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.values[key]; !ok {
		return nil
	}
	c.setTTLLocked(key, ttl)
	return nil
}

func (c *fakeBasicCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isExpiredLocked(key) {
		return -2, nil
	}
	exp, ok := c.expires[key]
	if !ok {
		if _, exists := c.values[key]; exists {
			return -1, nil
		}
		return -2, nil
	}
	return time.Until(exp), nil
}

func (c *fakeBasicCache) Incr(ctx context.Context, key string) (int64, error) {
	return c.IncrBy(ctx, key, 1)
}

func (c *fakeBasicCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.isExpiredLocked(key)
	current := parseInt(c.values[key])
	current += value
	c.values[key] = strconv.FormatInt(current, 10)
	return current, nil
}

func (c *fakeBasicCache) Decr(ctx context.Context, key string) (int64, error) {
	return c.DecrBy(ctx, key, 1)
}

func (c *fakeBasicCache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.IncrBy(ctx, key, -value)
}

func (c *fakeBasicCache) isExpiredLocked(key string) bool {
	exp, ok := c.expires[key]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(c.values, key)
		delete(c.expires, key)
		return true
	}
	return false
}

func (c *fakeBasicCache) setTTLLocked(key string, ttl time.Duration) {
	if ttl <= 0 {
		delete(c.expires, key)
		return
	}
	c.expires[key] = time.Now().Add(ttl)
}

func parseInt(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

type testDeps struct {
	users  *fakeUserRepo
	tokens *fakeTokenRepo
	cache  *fakeBasicCache
	svcCtx *svc.ServiceContext
}

func newTestDeps() *testDeps {
	users := newFakeUserRepo()
	tokens := newFakeTokenRepo()
	cache := newFakeBasicCache()
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
		Config:         cfg,
		Conn:           nil,
		UserRepo:       users,
		TokenRepo:      tokens,
		LoginFailCache: cache,
	}
	return &testDeps{
		users:  users,
		tokens: tokens,
		cache:  cache,
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

var _ commoncache.BasicOps = (*fakeBasicCache)(nil)
