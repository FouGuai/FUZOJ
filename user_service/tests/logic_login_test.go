package user_service_test

import (
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/user_service/internal/logic"
	"fuzoj/user_service/internal/repository"
	"fuzoj/user_service/internal/types"
)

func TestLoginLogicLoginSuccess(t *testing.T) {
	deps := newTestDeps()
	ctx := newTraceContext("trace-login")
	svcCtx := newServiceContext(deps.auth)
	loginLogic := logic.NewLoginLogic(ctx, svcCtx)

	user := &repository.User{
		Username:     "bob",
		Email:        "bob@local",
		PasswordHash: mustHashPassword(t, "Passw0rd!"),
		Role:         repository.UserRoleUser,
		Status:       repository.UserStatusActive,
	}
	if _, err := deps.users.Create(ctx, user); err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	resp, err := loginLogic.Login(&types.LoginRequest{
		Username:   "  bob  ",
		Password:   "Passw0rd!",
		IP:         "127.0.0.1",
		DeviceInfo: "unit-test",
	})
	if err != nil {
		t.Fatalf("expected login success, got error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.TraceId != "trace-login" {
		t.Fatalf("unexpected trace id: %s", resp.TraceId)
	}
	if resp.Data.User.Username != "bob" {
		t.Fatalf("unexpected username: %s", resp.Data.User.Username)
	}
	if resp.Data.AccessToken == "" || resp.Data.RefreshToken == "" {
		t.Fatalf("expected tokens in response")
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.Data.AccessExpiresAt); err != nil {
		t.Fatalf("invalid access expires time: %v", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, resp.Data.RefreshExpiresAt); err != nil {
		t.Fatalf("invalid refresh expires time: %v", err)
	}

	accessHash := hashToken(resp.Data.AccessToken)
	refreshHash := hashToken(resp.Data.RefreshToken)
	accessToken, err := deps.tokens.GetByHash(ctx, accessHash)
	if err != nil {
		t.Fatalf("expected access token record: %v", err)
	}
	refreshToken, err := deps.tokens.GetByHash(ctx, refreshHash)
	if err != nil {
		t.Fatalf("expected refresh token record: %v", err)
	}
	if accessToken.DeviceInfo != "unit-test" || refreshToken.DeviceInfo != "unit-test" {
		t.Fatalf("expected device info persisted")
	}
	if accessToken.IPAddress != "127.0.0.1" || refreshToken.IPAddress != "127.0.0.1" {
		t.Fatalf("expected ip persisted")
	}
}

func TestLoginLogicLoginInvalidCredentials(t *testing.T) {
	deps := newTestDeps()
	ctx := newTraceContext("trace-login-invalid")
	svcCtx := newServiceContext(deps.auth)
	loginLogic := logic.NewLoginLogic(ctx, svcCtx)

	user := &repository.User{
		Username:     "alice",
		Email:        "alice@local",
		PasswordHash: mustHashPassword(t, "Passw0rd!"),
		Role:         repository.UserRoleUser,
		Status:       repository.UserStatusActive,
	}
	if _, err := deps.users.Create(ctx, user); err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	resp, err := loginLogic.Login(&types.LoginRequest{
		Username: "alice",
		Password: "wrongpass1",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if resp != nil {
		t.Fatalf("expected nil response on error")
	}
	if !pkgerrors.Is(err, pkgerrors.InvalidCredentials) {
		t.Fatalf("unexpected error code: %v", pkgerrors.GetCode(err))
	}
}
