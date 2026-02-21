package logic

import (
	"context"
	"fmt"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/user_service/internal/service"
	"fuzoj/user_service/internal/types"
)

func buildAuthResponse(ctx context.Context, result service.AuthResult) *types.AuthResponse {
	return &types.AuthResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.AuthPayload{
			AccessToken:      result.AccessToken,
			RefreshToken:     result.RefreshToken,
			AccessExpiresAt:  formatTime(result.AccessExpiresAt),
			RefreshExpiresAt: formatTime(result.RefreshExpiresAt),
			User: types.UserInfo{
				Id:       result.User.ID,
				Username: result.User.Username,
				Role:     string(result.User.Role),
			},
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildSuccessResponse(ctx context.Context, message string) *types.SuccessResponse {
	if message == "" {
		message = "Success"
	}
	return &types.SuccessResponse{
		Code:    int(pkgerrors.Success),
		Message: message,
		TraceId: traceIDFromContext(ctx),
	}
}

func traceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID := ctx.Value(contextkey.TraceID); traceID != nil {
		return fmt.Sprint(traceID)
	}
	return ""
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
