package user_service_test

import (
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/user_service/internal/logic"
	"fuzoj/user_service/internal/types"
)

func TestRefreshLogicRefreshSuccess(t *testing.T) {
	deps := newTestDeps()
	ctx := newTraceContext("trace-refresh")
	svcCtx := newServiceContext(deps.auth)
	regLogic := logic.NewRegisterLogic(ctx, svcCtx)
	refreshLogic := logic.NewRefreshLogic(ctx, svcCtx)

	registerResp, err := regLogic.Register(&types.RegisterRequest{
		Username: "refreshuser",
		Password: "Passw0rd!",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	oldRefreshToken := registerResp.Data.RefreshToken

	resp, err := refreshLogic.Refresh(&types.RefreshRequest{
		RefreshToken: oldRefreshToken,
	})
	if err != nil {
		t.Fatalf("expected refresh success, got error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.TraceId != "trace-refresh" {
		t.Fatalf("unexpected trace id: %s", resp.TraceId)
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

	oldHash := hashToken(oldRefreshToken)
	tokenRecord, err := deps.tokens.GetByHash(ctx, oldHash)
	if err != nil {
		t.Fatalf("expected old refresh token record: %v", err)
	}
	if !tokenRecord.Revoked {
		t.Fatalf("expected old refresh token revoked")
	}
}

func TestRefreshLogicRefreshInvalidToken(t *testing.T) {
	deps := newTestDeps()
	ctx := newTraceContext("trace-refresh-invalid")
	svcCtx := newServiceContext(deps.auth)
	refreshLogic := logic.NewRefreshLogic(ctx, svcCtx)

	resp, err := refreshLogic.Refresh(&types.RefreshRequest{
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
