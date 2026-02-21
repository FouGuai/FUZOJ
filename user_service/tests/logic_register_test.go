package user_service_test

import (
	"testing"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/user_service/internal/logic"
	"fuzoj/user_service/internal/types"
)

func TestRegisterLogicRegister(t *testing.T) {
	deps := newTestDeps()
	ctx := newTraceContext("trace-register")
	svcCtx := newServiceContext(deps.auth)
	regLogic := logic.NewRegisterLogic(ctx, svcCtx)

	resp, err := regLogic.Register(&types.RegisterRequest{
		Username: "  Alice  ",
		Password: "Passw0rd!",
	})
	if err != nil {
		t.Fatalf("expected register success, got error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.Code != int(pkgerrors.Success) {
		t.Fatalf("unexpected response code: %d", resp.Code)
	}
	if resp.Message != "Success" {
		t.Fatalf("unexpected response message: %s", resp.Message)
	}
	if resp.TraceId != "trace-register" {
		t.Fatalf("unexpected trace id: %s", resp.TraceId)
	}
	if resp.Data.User.Username != "Alice" {
		t.Fatalf("unexpected username: %s", resp.Data.User.Username)
	}
	if deps.users.lastCreate == nil || deps.users.lastCreate.Username != "Alice" {
		t.Fatalf("expected trimmed username saved")
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
}

func TestRegisterLogicRegisterInvalidInput(t *testing.T) {
	deps := newTestDeps()
	ctx := newTraceContext("trace-register-invalid")
	svcCtx := newServiceContext(deps.auth)
	regLogic := logic.NewRegisterLogic(ctx, svcCtx)

	cases := []struct {
		name     string
		request  *types.RegisterRequest
		wantCode pkgerrors.ErrorCode
	}{
		{
			name: "invalid username",
			request: &types.RegisterRequest{
				Username: "  ab  ",
				Password: "Passw0rd!",
			},
			wantCode: pkgerrors.InvalidUsername,
		},
		{
			name: "weak password",
			request: &types.RegisterRequest{
				Username: "validuser",
				Password: "short1",
			},
			wantCode: pkgerrors.PasswordTooWeak,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := regLogic.Register(tc.request)
			if err == nil {
				t.Fatalf("expected error")
			}
			if resp != nil {
				t.Fatalf("expected nil response on error")
			}
			if !pkgerrors.Is(err, tc.wantCode) {
				t.Fatalf("unexpected error code: %v", pkgerrors.GetCode(err))
			}
		})
	}
}
