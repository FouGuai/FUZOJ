package response

import (
	"fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Response represents a standard API response
type Response struct {
	Code    errors.ErrorCode `json:"code"`              // Error code
	Message string           `json:"message"`           // Error message
	Data    interface{}      `json:"data,omitempty"`    // Response data (omit if nil)
	Details interface{}      `json:"details,omitempty"` // Additional details (omit if nil)
	TraceID string           `json:"trace_id,omitempty"` // Request trace ID
}

// Success sends a successful response with data
func Success(c *gin.Context, data interface{}) {
	resp := Response{
		Code:    errors.Success,
		Message: "Success",
		Data:    data,
		TraceID: getTraceID(c),
	}
	c.JSON(http.StatusOK, resp)
}

// SuccessWithMessage sends a successful response with custom message
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	resp := Response{
		Code:    errors.Success,
		Message: message,
		Data:    data,
		TraceID: getTraceID(c),
	}
	c.JSON(http.StatusOK, resp)
}

// Error sends an error response
// It automatically extracts error code and message from the error
func Error(c *gin.Context, err error) {
	customErr := errors.GetError(err)
	
	// Log the error with context
	logger.Error(c.Request.Context(), "request error",
		zap.Int("code", int(customErr.Code)),
		zap.String("message", customErr.Error()),
		zap.Any("details", customErr.Details),
		zap.String("stack", customErr.Stack),
	)

	resp := Response{
		Code:    customErr.Code,
		Message: customErr.Error(),
		Details: customErr.Details,
		TraceID: getTraceID(c),
	}

	c.JSON(customErr.Code.HTTPStatus(), resp)
}

// ErrorWithCode sends an error response with specific error code
func ErrorWithCode(c *gin.Context, code errors.ErrorCode, message string) {
	if message == "" {
		message = code.Message()
	}

	logger.Error(c.Request.Context(), "request error",
		zap.Int("code", int(code)),
		zap.String("message", message),
	)

	resp := Response{
		Code:    code,
		Message: message,
		TraceID: getTraceID(c),
	}

	c.JSON(code.HTTPStatus(), resp)
}

// ErrorWithDetails sends an error response with additional details
func ErrorWithDetails(c *gin.Context, err error, details interface{}) {
	customErr := errors.GetError(err)

	logger.Error(c.Request.Context(), "request error",
		zap.Int("code", int(customErr.Code)),
		zap.String("message", customErr.Error()),
		zap.Any("details", details),
		zap.String("stack", customErr.Stack),
	)

	resp := Response{
		Code:    customErr.Code,
		Message: customErr.Error(),
		Details: details,
		TraceID: getTraceID(c),
	}

	c.JSON(customErr.Code.HTTPStatus(), resp)
}

// BadRequest sends a 400 bad request error
func BadRequest(c *gin.Context, message string) {
	ErrorWithCode(c, errors.InvalidParams, message)
}

// Unauthorized sends a 401 unauthorized error
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = errors.Unauthorized.Message()
	}
	ErrorWithCode(c, errors.Unauthorized, message)
}

// Forbidden sends a 403 forbidden error
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = errors.Forbidden.Message()
	}
	ErrorWithCode(c, errors.Forbidden, message)
}

// NotFound sends a 404 not found error
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = errors.NotFound.Message()
	}
	ErrorWithCode(c, errors.NotFound, message)
}

// InternalServerError sends a 500 internal server error
func InternalServerError(c *gin.Context, err error) {
	customErr := errors.InternalError(err)
	Error(c, customErr)
}

// Paginated represents a paginated response
type Paginated struct {
	Items      interface{} `json:"items"`       // List of items
	Total      int64       `json:"total"`       // Total count
	Page       int         `json:"page"`        // Current page
	PageSize   int         `json:"page_size"`   // Page size
	TotalPages int         `json:"total_pages"` // Total pages
}

// SuccessWithPagination sends a successful response with pagination
func SuccessWithPagination(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	data := Paginated{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	Success(c, data)
}

// getTraceID extracts trace ID from context
func getTraceID(c *gin.Context) string {
	if traceID, exists := c.Get("trace_id"); exists {
		return traceID.(string)
	}
	return ""
}

// AbortWithError aborts the request and sends error response
func AbortWithError(c *gin.Context, err error) {
	Error(c, err)
	c.Abort()
}

// AbortWithErrorCode aborts the request with error code
func AbortWithErrorCode(c *gin.Context, code errors.ErrorCode, message string) {
	ErrorWithCode(c, code, message)
	c.Abort()
}
