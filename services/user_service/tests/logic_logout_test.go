package user_service_test

import (
	"testing"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/user_service/internal/logic"
	"fuzoj/services/user_service/internal/types"
)

func TestLogoutLogicLogoutSuccess(t *testing.T) {
	deps := newTestDeps()
	ctx := newTraceContext("trace-logout")
	svcCtx := newServiceContext(deps)
	regLogic := logic.NewRegisterLogic(ctx, svcCtx)
	logoutLogic := logic.NewLogoutLogic(ctx, svcCtx)

	registerResp, err := regLogic.Register(&types.RegisterRequest{
		Username: "logoutuser",
		Password: "Passw0rd!",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	refreshToken := registerResp.Data.RefreshToken

	resp, err := logoutLogic.Logout(&types.LogoutRequest{
		RefreshToken: refreshToken,
	})
	if err != nil {
		t.Fatalf("expected logout success, got error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.Message != "Logout success" {
		t.Fatalf("unexpected response message: %s", resp.Message)
	}
	if resp.TraceId != "trace-logout" {
		t.Fatalf("unexpected trace id: %s", resp.TraceId)
	}

	hash := hashToken(refreshToken)
	tokenRecord, err := deps.tokens.GetByHash(ctx, hash)
	if err != nil {
		t.Fatalf("expected refresh token record: %v", err)
	}
	if !tokenRecord.Revoked {
		t.Fatalf("expected refresh token revoked")
	}
}

func TestLogoutLogicLogoutInvalidToken(t *testing.T) {
	deps := newTestDeps()
	ctx := newTraceContext("trace-logout-invalid")
	svcCtx := newServiceContext(deps)
	logoutLogic := logic.NewLogoutLogic(ctx, svcCtx)

	resp, err := logoutLogic.Logout(&types.LogoutRequest{
		RefreshToken: "",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if resp != nil {
		t.Fatalf("expected nil response on error")
	}
	if !pkgerrors.Is(err, pkgerrors.TokenInvalid) {
		t.Fatalf("unexpected error code: %v", pkgerrors.GetCode(err))
	}
}
