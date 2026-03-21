package logic

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/services/problem_service/internal/logic/problem_app"
	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/types"
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

func buildCreateResponse(ctx context.Context, id int64) *types.CreateProblemResponse {
	return &types.CreateProblemResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.CreateProblemPayload{
			Id: id,
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildLatestMetaResponse(ctx context.Context, meta repository.ProblemLatestMeta) *types.LatestMetaResponse {
	return &types.LatestMetaResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.LatestMetaPayload{
			ProblemId:    meta.ProblemID,
			Version:      meta.Version,
			ManifestHash: meta.ManifestHash,
			DataPackKey:  meta.DataPackKey,
			DataPackHash: meta.DataPackHash,
			UpdatedAt:    formatTime(meta.UpdatedAt),
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildListProblemsResponse(ctx context.Context, items []repository.ProblemListItem, hasMore bool) *types.ListProblemsResponse {
	respItems := make([]types.ListProblemItem, 0, len(items))
	nextCursor := ""
	for _, item := range items {
		respItems = append(respItems, types.ListProblemItem{
			ProblemId: item.ProblemID,
			Title:     item.Title,
			Version:   item.Version,
			UpdatedAt: formatTime(item.UpdatedAt),
		})
		nextCursor = strconv.FormatInt(item.ProblemID, 10)
	}
	if !hasMore {
		nextCursor = ""
	}
	return &types.ListProblemsResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.ListProblemsPayload{
			Items:      respItems,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildStatementResponse(ctx context.Context, statement repository.ProblemStatement) *types.StatementResponse {
	return &types.StatementResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.StatementPayload{
			ProblemId:   statement.ProblemID,
			Version:     statement.Version,
			StatementMd: statement.StatementMd,
			UpdatedAt:   formatTime(statement.UpdatedAt),
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildPrepareUploadResponse(ctx context.Context, output problem_app.PrepareUploadOutput) *types.PrepareUploadResponse {
	return &types.PrepareUploadResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.PrepareUploadPayload{
			UploadId:          output.UploadSessionID,
			ProblemId:         output.ProblemID,
			Version:           output.Version,
			Bucket:            output.Bucket,
			ObjectKey:         output.ObjectKey,
			MultipartUploadId: output.MultipartUploadID,
			PartSizeBytes:     output.PartSizeBytes,
			ExpiresAt:         formatTime(output.ExpiresAt),
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildSignPartsResponse(ctx context.Context, output problem_app.SignPartsOutput) *types.SignPartsResponse {
	urls := make(map[string]string, len(output.URLs))
	for key, value := range output.URLs {
		urls[strconv.Itoa(key)] = value
	}
	return &types.SignPartsResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.SignPartsPayload{
			Urls:             urls,
			ExpiresInSeconds: output.ExpiresInSeconds,
		},
		TraceId: traceIDFromContext(ctx),
	}
}

func buildCompleteUploadResponse(ctx context.Context, output problem_app.CompleteUploadOutput) *types.CompleteUploadResponse {
	return &types.CompleteUploadResponse{
		Code:    int(pkgerrors.Success),
		Message: "Success",
		Data: types.CompleteUploadPayload{
			ProblemId:    output.ProblemID,
			Version:      output.Version,
			ManifestHash: output.ManifestHash,
			DataPackKey:  output.DataPackKey,
			DataPackHash: output.DataPackHash,
		},
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
