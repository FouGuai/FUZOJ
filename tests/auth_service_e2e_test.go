package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
	"fuzoj/internal/user/controller"
	"fuzoj/internal/user/repository"
	"fuzoj/internal/user/service"
	pkgerrors "fuzoj/pkg/errors"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

const (
	loginFailUserPrefix   = "login:fail:username:"
	loginFailIPPrefix     = "login:fail:ip:"
	tokenBlacklistKey     = "token:blacklist"
	tokenCacheKeyPrefix   = "token:hash:"
	testConfigPath        = "./test.yaml"
	schemaFilePath        = "../internal/user/schema.sql"
	defaultTestConfigYAML = `mysql:
  dsn: "user:password@tcp(127.0.0.1:3306)/fuzoj_test?parseTime=true&loc=Local"
  maxOpenConnections: 25
  maxIdleConnections: 5
  connMaxLifetime: "5m"
  connMaxIdleTime: "10m"
redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  maxRetries: 3
  minRetryBackoff: "8ms"
  maxRetryBackoff: "512ms"
  dialTimeout: "5s"
  readTimeout: "3s"
  writeTimeout: "3s"
  poolSize: 20
  minIdleConns: 2
  poolTimeout: "4s"
  connMaxIdleTime: "10m"
  connMaxLifetime: "30m"
`
)

type testConfig struct {
	MySQL db.MySQLConfig    `yaml:"mysql"`
	Redis cache.RedisConfig `yaml:"redis"`
}

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Details json.RawMessage `json:"details"`
}

func TestAuthService_EndToEnd(t *testing.T) {
	cfg := loadTestConfig(t)
	gin.SetMode(gin.TestMode)

	mysqlDB, redisCache := newE2EDeps(t, cfg)
	if mysqlDB == nil || redisCache == nil {
		return
	}
	defer func() {
		_ = mysqlDB.Close()
		_ = redisCache.Close()
	}()

	ensureSchema(t, mysqlDB)

	router := setupAuthRouter(mysqlDB, redisCache)

	username := fmt.Sprintf("e2e_%d", time.Now().UnixNano()%1_000_000_000)
	password := "password123"
	clientIP := "127.0.0.1"
	userAgent := "e2e-test"

	registerResp := doPostJSON(t, router, "/api/v1/user/register", map[string]string{
		"username": username,
		"password": password,
	}, map[string]string{"User-Agent": userAgent})
	assertSuccessResponse(t, registerResp, "register")
	registerData := mustDecodeAuthResponse(t, registerResp.Data)
	if registerData.User.Username != username {
		t.Fatalf("register username mismatch: got %s want %s", registerData.User.Username, username)
	}
	if registerData.User.ID == 0 {
		t.Fatalf("register user id should be set")
	}

	userID := registerData.User.ID
	email := fmt.Sprintf("%s@local", username)
	tokens := []string{registerData.AccessToken, registerData.RefreshToken}
	cleanup := func() {
		cleanupE2EData(t, mysqlDB, redisCache, userID, username, email, tokens, clientIP)
	}
	t.Cleanup(cleanup)

	userIDFromDB, status, role, passwordHash := fetchUserByUsername(t, mysqlDB, username)
	if userIDFromDB != userID {
		t.Fatalf("user id mismatch: got %d want %d", userIDFromDB, userID)
	}
	if status != string(repository.UserStatusActive) {
		t.Fatalf("user status mismatch: got %s want %s", status, repository.UserStatusActive)
	}
	if role != string(repository.UserRoleUser) {
		t.Fatalf("user role mismatch: got %s want %s", role, repository.UserRoleUser)
	}
	if passwordHash == "" {
		t.Fatalf("password hash should not be empty")
	}

	count := countTokensByUserID(t, mysqlDB, userID)
	if count != 2 {
		t.Fatalf("token count after register mismatch: got %d want %d", count, 2)
	}

	loginResp := doPostJSON(t, router, "/api/v1/user/login", map[string]string{
		"username": username,
		"password": password,
	}, map[string]string{"User-Agent": userAgent, "X-Forwarded-For": clientIP})
	assertSuccessResponse(t, loginResp, "login")
	loginData := mustDecodeAuthResponse(t, loginResp.Data)
	if loginData.User.ID != userID {
		t.Fatalf("login user id mismatch: got %d want %d", loginData.User.ID, userID)
	}
	if loginData.AccessToken == "" || loginData.RefreshToken == "" {
		t.Fatalf("login tokens should not be empty")
	}

	tokens = append(tokens, loginData.AccessToken, loginData.RefreshToken)
	count = countTokensByUserID(t, mysqlDB, userID)
	if count != 4 {
		t.Fatalf("token count after login mismatch: got %d want %d", count, 4)
	}

	loginRefreshHash := hashTokenForTest(loginData.RefreshToken)
	if revoked := isTokenRevoked(t, mysqlDB, loginRefreshHash); revoked {
		t.Fatalf("login refresh token should not be revoked")
	}

	refreshResp := doPostJSON(t, router, "/api/v1/user/refresh-token", map[string]string{
		"refresh_token": loginData.RefreshToken,
	}, nil)
	assertSuccessResponse(t, refreshResp, "refresh")
	refreshData := mustDecodeAuthResponse(t, refreshResp.Data)
	if refreshData.RefreshToken == loginData.RefreshToken {
		t.Fatalf("refresh token should rotate")
	}

	tokens = append(tokens, refreshData.AccessToken, refreshData.RefreshToken)
	count = countTokensByUserID(t, mysqlDB, userID)
	if count != 6 {
		t.Fatalf("token count after refresh mismatch: got %d want %d", count, 6)
	}

	if revoked := isTokenRevoked(t, mysqlDB, loginRefreshHash); !revoked {
		t.Fatalf("old refresh token should be revoked")
	}
	assertTokenBlacklisted(t, redisCache, loginRefreshHash, true)

	logoutResp := doPostJSON(t, router, "/api/v1/user/logout", map[string]string{
		"refresh_token": refreshData.RefreshToken,
	}, nil)
	assertSuccessResponse(t, logoutResp, "logout")

	newRefreshHash := hashTokenForTest(refreshData.RefreshToken)
	if revoked := isTokenRevoked(t, mysqlDB, newRefreshHash); !revoked {
		t.Fatalf("logout refresh token should be revoked")
	}
	assertTokenBlacklisted(t, redisCache, newRefreshHash, true)
}

func TestAuthService_EdgeCases(t *testing.T) {
	cfg := loadTestConfig(t)
	gin.SetMode(gin.TestMode)

	mysqlDB, redisCache := newE2EDeps(t, cfg)
	if mysqlDB == nil || redisCache == nil {
		return
	}
	defer func() {
		_ = mysqlDB.Close()
		_ = redisCache.Close()
	}()

	ensureSchema(t, mysqlDB)
	router := setupAuthRouter(mysqlDB, redisCache)

	t.Run("register validation", func(t *testing.T) {
		cases := []struct {
			name    string
			payload map[string]interface{}
			want    pkgerrors.ErrorCode
		}{
			{
				name:    "missing username",
				payload: map[string]interface{}{"password": "password123"},
				want:    pkgerrors.InvalidParams,
			},
			{
				name:    "null username",
				payload: map[string]interface{}{"username": nil, "password": "password123"},
				want:    pkgerrors.InvalidParams,
			},
			{
				name:    "empty username",
				payload: map[string]interface{}{"username": "", "password": "password123"},
				want:    pkgerrors.InvalidParams,
			},
			{
				name:    "invalid username",
				payload: map[string]interface{}{"username": "bad name", "password": "password123"},
				want:    pkgerrors.InvalidUsername,
			},
			{
				name:    "short password",
				payload: map[string]interface{}{"username": "edgeuser", "password": "short"},
				want:    pkgerrors.PasswordTooWeak,
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				resp := doPostJSON(t, router, "/api/v1/user/register", tc.payload, nil)
				assertErrorCode(t, resp, tc.want, "register")
			})
		}
	})

	t.Run("duplicate username", func(t *testing.T) {
		username := fmt.Sprintf("dup_%d", time.Now().UnixNano()%1_000_000_000)
		_, _ = registerUserAndCleanup(t, router, mysqlDB, redisCache, username, "password123", "e2e-test", "127.0.0.1")

		resp := doPostJSON(t, router, "/api/v1/user/register", map[string]string{
			"username": username,
			"password": "password123",
		}, nil)
		assertErrorCode(t, resp, pkgerrors.UsernameAlreadyExists, "register duplicate")
	})

	t.Run("login validation", func(t *testing.T) {
		username := fmt.Sprintf("login_%d", time.Now().UnixNano()%1_000_000_000)
		password := "password123"
		_, _ = registerUserAndCleanup(t, router, mysqlDB, redisCache, username, password, "e2e-test", "127.0.0.1")

		cases := []struct {
			name    string
			payload map[string]interface{}
			want    pkgerrors.ErrorCode
		}{
			{
				name:    "missing username",
				payload: map[string]interface{}{"password": password},
				want:    pkgerrors.InvalidParams,
			},
			{
				name:    "null username",
				payload: map[string]interface{}{"username": nil, "password": password},
				want:    pkgerrors.InvalidParams,
			},
			{
				name:    "empty username",
				payload: map[string]interface{}{"username": "", "password": password},
				want:    pkgerrors.InvalidParams,
			},
			{
				name:    "short password",
				payload: map[string]interface{}{"username": username, "password": "short"},
				want:    pkgerrors.PasswordTooWeak,
			},
			{
				name:    "wrong password",
				payload: map[string]interface{}{"username": username, "password": "password124"},
				want:    pkgerrors.InvalidCredentials,
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				resp := doPostJSON(t, router, "/api/v1/user/login", tc.payload, nil)
				assertErrorCode(t, resp, tc.want, "login")
			})
		}
	})

	t.Run("login twice keeps previous tokens", func(t *testing.T) {
		username := fmt.Sprintf("twice_%d", time.Now().UnixNano()%1_000_000_000)
		password := "password123"
		clientIP := "127.0.0.1"
		userAgent := "e2e-test"

		registerResp, tokens := registerUserAndCleanup(t, router, mysqlDB, redisCache, username, password, userAgent, clientIP)
		userID := registerResp.User.ID

		loginResp1 := doPostJSON(t, router, "/api/v1/user/login", map[string]string{
			"username": username,
			"password": password,
		}, map[string]string{"User-Agent": userAgent, "X-Forwarded-For": clientIP})
		assertSuccessResponse(t, loginResp1, "login first")
		loginData1 := mustDecodeAuthResponse(t, loginResp1.Data)
		tokens = append(tokens, loginData1.AccessToken, loginData1.RefreshToken)

		loginResp2 := doPostJSON(t, router, "/api/v1/user/login", map[string]string{
			"username": username,
			"password": password,
		}, map[string]string{"User-Agent": userAgent, "X-Forwarded-For": clientIP})
		assertSuccessResponse(t, loginResp2, "login second")
		loginData2 := mustDecodeAuthResponse(t, loginResp2.Data)
		tokens = append(tokens, loginData2.AccessToken, loginData2.RefreshToken)

		count := countTokensByUserID(t, mysqlDB, userID)
		if count != 6 {
			t.Fatalf("token count after double login mismatch: got %d want %d", count, 6)
		}

		firstRefreshHash := hashTokenForTest(loginData1.RefreshToken)
		if revoked := isTokenRevoked(t, mysqlDB, firstRefreshHash); revoked {
			t.Fatalf("first login refresh token should not be revoked")
		}

		secondRefreshHash := hashTokenForTest(loginData2.RefreshToken)
		if revoked := isTokenRevoked(t, mysqlDB, secondRefreshHash); revoked {
			t.Fatalf("second login refresh token should not be revoked")
		}
	})

	t.Run("multi user login", func(t *testing.T) {
		userAgent := "e2e-test"

		usernameA := fmt.Sprintf("usera_%d", time.Now().UnixNano()%1_000_000_000)
		respA, tokensA := registerUserAndCleanup(t, router, mysqlDB, redisCache, usernameA, "password123", userAgent, "127.0.0.1")

		usernameB := fmt.Sprintf("userb_%d", time.Now().UnixNano()%1_000_000_000)
		respB, tokensB := registerUserAndCleanup(t, router, mysqlDB, redisCache, usernameB, "password123", userAgent, "127.0.0.2")

		loginRespA := doPostJSON(t, router, "/api/v1/user/login", map[string]string{
			"username": usernameA,
			"password": "password123",
		}, map[string]string{"User-Agent": userAgent})
		assertSuccessResponse(t, loginRespA, "login user A")
		loginDataA := mustDecodeAuthResponse(t, loginRespA.Data)
		tokensA = append(tokensA, loginDataA.AccessToken, loginDataA.RefreshToken)

		countA := countTokensByUserID(t, mysqlDB, respA.User.ID)
		if countA != 4 {
			t.Fatalf("token count for user A mismatch: got %d want %d", countA, 4)
		}

		loginRespB := doPostJSON(t, router, "/api/v1/user/login", map[string]string{
			"username": usernameB,
			"password": "password123",
		}, map[string]string{"User-Agent": userAgent})
		assertSuccessResponse(t, loginRespB, "login user B")
		loginDataB := mustDecodeAuthResponse(t, loginRespB.Data)
		tokensB = append(tokensB, loginDataB.AccessToken, loginDataB.RefreshToken)

		countB := countTokensByUserID(t, mysqlDB, respB.User.ID)
		if countB != 4 {
			t.Fatalf("token count for user B mismatch: got %d want %d", countB, 4)
		}

		countA = countTokensByUserID(t, mysqlDB, respA.User.ID)
		if countA != 4 {
			t.Fatalf("token count for user A after user B login mismatch: got %d want %d", countA, 4)
		}
	})
}

func loadTestConfig(t *testing.T) *testConfig {
	t.Helper()

	data, err := os.ReadFile(testConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			ensureDefaultTestConfig(t, testConfigPath)
			data, err = os.ReadFile(testConfigPath)
		}
		if err != nil {
			t.Fatalf("read test config failed: %v", err)
		}
	}

	var cfg testConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse test config failed: %v", err)
	}

	if cfg.MySQL.DSN == "" {
		t.Fatalf("mysql.dsn is required in %s", testConfigPath)
	}
	if cfg.Redis.Addr == "" {
		t.Fatalf("redis.addr is required in %s", testConfigPath)
	}

	return &cfg
}

func ensureDefaultTestConfig(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(defaultTestConfigYAML), 0o644); err != nil {
		t.Fatalf("write default test config failed: %v", err)
	}
}

func newE2EDeps(t *testing.T, cfg *testConfig) (*db.MySQL, cache.Cache) {
	t.Helper()

	mysqlDB, err := db.NewMySQLWithConfig(&cfg.MySQL)
	if err != nil {
		t.Fatalf("connect mysql failed: %v", err)
	}

	redisCache, err := cache.NewRedisCacheWithConfig(&cfg.Redis)
	if err != nil {
		_ = mysqlDB.Close()
		t.Fatalf("connect redis failed: %v", err)
	}

	return mysqlDB, redisCache
}

func ensureSchema(t *testing.T, database db.Database) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	statements := loadSchemaStatements(t)

	for _, stmt := range statements {
		if _, err := database.Exec(ctx, stmt); err != nil {
			t.Fatalf("create table failed: %v", err)
		}
	}
}

func loadSchemaStatements(t *testing.T) []string {
	t.Helper()

	absPath, err := filepath.Abs(schemaFilePath)
	if err != nil {
		t.Fatalf("resolve schema path failed: %v", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("read schema file failed: %v", err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		t.Fatalf("schema file is empty: %s", absPath)
	}

	lines := make([]string, 0, 128)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if idx := strings.Index(trimmed, "--"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	joined := strings.Join(lines, "\n")
	parts := strings.Split(joined, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		statements = append(statements, stmt)
	}

	if len(statements) == 0 {
		t.Fatalf("no schema statements loaded from: %s", absPath)
	}

	return statements
}

func setupAuthRouter(mysqlDB db.Database, redisCache cache.Cache) http.Handler {
	dbProvider := db.NewStaticProvider(mysqlDB)
	userRepo := repository.NewUserRepository(dbProvider, redisCache)
	tokenRepo := repository.NewTokenRepository(dbProvider, redisCache)
	authService := service.NewAuthService(dbProvider, userRepo, tokenRepo, redisCache, service.AuthServiceConfig{
		JWTSecret:       []byte("e2e-secret"),
		JWTIssuer:       "fuzoj-e2e",
		AccessTokenTTL:  10 * time.Minute,
		RefreshTokenTTL: time.Hour,
		LoginFailTTL:    15 * time.Minute,
		LoginFailLimit:  5,
	})

	router := gin.New()
	api := router.Group("/api/v1/user")
	authController := controller.NewAuthController(authService)
	api.POST("/register", authController.Register)
	api.POST("/login", authController.Login)
	api.POST("/refresh-token", authController.Refresh)
	api.POST("/logout", authController.Logout)
	return router
}

func doPostJSON(t *testing.T, handler http.Handler, path string, payload interface{}, headers map[string]string) apiResponse {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if headers != nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
	req.RemoteAddr = "127.0.0.1:12345"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	var resp apiResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	return resp
}

func assertSuccessResponse(t *testing.T, resp apiResponse, step string) {
	t.Helper()
	if resp.Code != int(pkgerrors.Success) {
		t.Fatalf("%s failed: code=%d message=%s", step, resp.Code, resp.Message)
	}
}

func assertErrorCode(t *testing.T, resp apiResponse, want pkgerrors.ErrorCode, step string) {
	t.Helper()
	if resp.Code != int(want) {
		t.Fatalf("%s failed: code=%d want=%d message=%s", step, resp.Code, want, resp.Message)
	}
}

func mustDecodeAuthResponse(t *testing.T, data json.RawMessage) controller.AuthResponse {
	t.Helper()
	if len(data) == 0 {
		t.Fatalf("response data is empty")
	}
	var authResp controller.AuthResponse
	if err := json.Unmarshal(data, &authResp); err != nil {
		t.Fatalf("decode auth response failed: %v", err)
	}
	return authResp
}

func registerUserAndCleanup(
	t *testing.T,
	router http.Handler,
	mysqlDB db.Database,
	redisCache cache.Cache,
	username string,
	password string,
	userAgent string,
	clientIP string,
) (controller.AuthResponse, []string) {
	t.Helper()

	resp := doPostJSON(t, router, "/api/v1/user/register", map[string]string{
		"username": username,
		"password": password,
	}, map[string]string{"User-Agent": userAgent, "X-Forwarded-For": clientIP})
	assertSuccessResponse(t, resp, "register")

	data := mustDecodeAuthResponse(t, resp.Data)
	tokens := []string{data.AccessToken, data.RefreshToken}
	email := fmt.Sprintf("%s@local", username)

	t.Cleanup(func() {
		cleanupE2EData(t, mysqlDB, redisCache, data.User.ID, username, email, tokens, clientIP)
	})

	return data, tokens
}

func fetchUserByUsername(t *testing.T, database db.Database, username string) (int64, string, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := database.QueryRow(ctx, "SELECT id, status, role, password_hash FROM users WHERE username = ?", username)
	var id int64
	var status, role, passwordHash string
	if err := row.Scan(&id, &status, &role, &passwordHash); err != nil {
		t.Fatalf("query user failed: %v", err)
	}
	return id, status, role, passwordHash
}

func countTokensByUserID(t *testing.T, database db.Database, userID int64) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := database.QueryRow(ctx, "SELECT COUNT(*) FROM user_tokens WHERE user_id = ?", userID)
	var count int64
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count tokens failed: %v", err)
	}
	return count
}

func isTokenRevoked(t *testing.T, database db.Database, tokenHash string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := database.QueryRow(ctx, "SELECT revoked FROM user_tokens WHERE token_hash = ?", tokenHash)
	var revoked bool
	if err := row.Scan(&revoked); err != nil {
		t.Fatalf("query token revoked failed: %v", err)
	}
	return revoked
}

func assertTokenBlacklisted(t *testing.T, redisCache cache.Cache, tokenHash string, want bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blacklisted, err := redisCache.SIsMember(ctx, tokenBlacklistKey, tokenHash)
	if err != nil {
		t.Fatalf("check token blacklist failed: %v", err)
	}
	if blacklisted != want {
		t.Fatalf("token blacklist mismatch: got %v want %v", blacklisted, want)
	}
}

func cleanupE2EData(t *testing.T, mysqlDB db.Database, redisCache cache.Cache, userID int64, username, email string, tokens []string, ip string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = mysqlDB.Exec(ctx, "DELETE FROM user_tokens WHERE user_id = ?", userID)
	_, _ = mysqlDB.Exec(ctx, "DELETE FROM users WHERE id = ?", userID)

	if redisCache == nil {
		return
	}

	keys := []string{
		fmt.Sprintf("user:info:%d", userID),
		"user:username:" + username,
		"user:email:" + email,
		loginFailUserPrefix + username,
	}
	if ip != "" {
		keys = append(keys, loginFailIPPrefix+ip)
	}
	for _, token := range tokens {
		hash := hashTokenForTest(token)
		keys = append(keys, tokenCacheKeyPrefix+hash)
	}
	_ = redisCache.Del(ctx, keys...)

	hashes := make([]interface{}, 0, len(tokens))
	for _, token := range tokens {
		hashes = append(hashes, hashTokenForTest(token))
	}
	if len(hashes) > 0 {
		_ = redisCache.SRem(ctx, tokenBlacklistKey, hashes...)
	}
}
