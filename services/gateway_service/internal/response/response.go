package response

import (
	"encoding/json"
	"net/http"

	"fuzoj/pkg/errors"
	"fuzoj/pkg/utils/contextkey"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

// Response represents a standard API response.
type Response struct {
	Code    errors.ErrorCode `json:"code"`
	Message string           `json:"message"`
	Data    interface{}      `json:"data,omitempty"`
	Details interface{}      `json:"details,omitempty"`
	TraceID string           `json:"trace_id,omitempty"`
}

// WriteError writes an error response in the standard envelope.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	customErr := errors.GetError(err)

	logger.Error(r.Context(), "request error",
		zap.Int("code", int(customErr.Code)),
		zap.String("message", customErr.Error()),
		zap.Any("details", customErr.Details),
		zap.String("stack", customErr.Stack),
		debugLocationField(),
	)

	resp := Response{
		Code:    customErr.Code,
		Message: customErr.Error(),
		Details: customErr.Details,
		TraceID: getTraceID(r),
	}
	writeJSON(w, customErr.Code.HTTPStatus(), resp)
}

// WriteErrorCode writes an error response with a specific error code.
func WriteErrorCode(w http.ResponseWriter, r *http.Request, code errors.ErrorCode, message string) {
	if message == "" {
		message = code.Message()
	}
	logger.Error(r.Context(), "request error",
		zap.Int("code", int(code)),
		zap.String("message", message),
		debugLocationField(),
	)

	resp := Response{
		Code:    code,
		Message: message,
		TraceID: getTraceID(r),
	}
	writeJSON(w, code.HTTPStatus(), resp)
}

func getTraceID(r *http.Request) string {
	if val := r.Context().Value(contextkey.TraceID); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
		return ""
	}
	return ""
}

func writeJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func debugLocationField() zap.Field {
	if logger.IsDebug() {
		return logger.CallerField(3)
	}
	return zap.Skip()
}
