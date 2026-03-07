package logic

import (
	"context"
	"fmt"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/services/contest_service/internal/types"
)

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

func buildCreateContestResponse(ctx context.Context, contestID string) *types.CreateContestResponse {
	return &types.CreateContestResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.CreateContestPayload{
			ContestId: contestID,
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildGetContestResponse(ctx context.Context, detail types.ContestDetail) *types.GetContestResponse {
	return &types.GetContestResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data:    detail,
		TraceId: traceIDFromContext(ctx),
	}
}

func buildListContestsResponse(ctx context.Context, payload types.ListContestsPayload) *types.ListContestsResponse {
	return &types.ListContestsResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data:    payload,
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
	return value.UTC().Format(time.RFC3339)
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}
