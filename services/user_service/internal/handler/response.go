package handler

import (
	"context"
	"fmt"
	"net/http"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/pkg/utils/logger"

	"github.com/zeromicro/go-zero/rest/httpx"
	"go.uber.org/zap"
)

type errorResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	TraceId string            `json:"trace_id,omitempty"`
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	customErr := pkgerrors.GetError(err)

	logger.Error(r.Context(), "request error",
		zap.Int("code", int(customErr.Code)),
		zap.String("message", customErr.Error()),
		zap.Any("details", customErr.Details),
		zap.String("stack", customErr.Stack),
		debugLocationField(),
	)

	resp := errorResponse{
		Code:    int(customErr.Code),
		Message: customErr.Error(),
		Details: stringifyDetails(customErr.Details),
		TraceId: traceIDFromContext(r.Context()),
	}

	httpx.WriteJsonCtx(r.Context(), w, customErr.Code.HTTPStatus(), resp)
}

func badRequestError() error {
	return pkgerrors.BadRequest("Invalid request parameters")
}

func stringifyDetails(details map[string]interface{}) map[string]string {
	if len(details) == 0 {
		return nil
	}
	result := make(map[string]string, len(details))
	for key, value := range details {
		result[key] = fmt.Sprint(value)
	}
	return result
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

func debugLocationField() zap.Field {
	if logger.IsDebug() {
		return logger.CallerField(3)
	}
	return zap.Skip()
}
