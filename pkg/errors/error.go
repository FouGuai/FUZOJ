package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// Error represents a custom error with error code and context
type Error struct {
	Code    ErrorCode              // Error code
	Message string                 // Custom error message (overrides default if set)
	Details map[string]interface{} // Additional context data
	Err     error                  // Underlying error (for wrapping)
	Stack   string                 // Stack trace
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code.Message()
}

// Unwrap returns the underlying error (for errors.Is and errors.As)
func (e *Error) Unwrap() error {
	return e.Err
}

// New creates a new Error with the given error code
func New(code ErrorCode) *Error {
	return &Error{
		Code:    code,
		Message: code.Message(),
		Details: make(map[string]interface{}),
		Stack:   getStack(2),
	}
}

// Newf creates a new Error with formatted message
func Newf(code ErrorCode, format string, args ...interface{}) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Details: make(map[string]interface{}),
		Stack:   getStack(2),
	}
}

// Wrap wraps an existing error with an error code
func Wrap(err error, code ErrorCode) *Error {
	if err == nil {
		return nil
	}

	// If already our custom error, just update the code
	if e, ok := err.(*Error); ok {
		e.Code = code
		return e
	}

	return &Error{
		Code:    code,
		Message: err.Error(),
		Err:     err,
		Details: make(map[string]interface{}),
		Stack:   getStack(2),
	}
}

// Wrapf wraps an error with code and formatted message
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *Error {
	if err == nil {
		return nil
	}

	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
		Details: make(map[string]interface{}),
		Stack:   getStack(2),
	}
}

// WithMessage adds a custom message to the error
func (e *Error) WithMessage(msg string) *Error {
	e.Message = msg
	return e
}

// WithMessagef adds a formatted custom message to the error
func (e *Error) WithMessagef(format string, args ...interface{}) *Error {
	e.Message = fmt.Sprintf(format, args...)
	return e
}

// WithDetail adds a key-value detail to the error
func (e *Error) WithDetail(key string, value interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithDetails adds multiple key-value details to the error
func (e *Error) WithDetails(details map[string]interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// GetCode extracts the error code from any error
// If the error is not our custom Error type, returns InternalServerError
func GetCode(err error) ErrorCode {
	if err == nil {
		return Success
	}

	if e, ok := err.(*Error); ok {
		return e.Code
	}

	return InternalServerError
}

// GetError extracts our custom Error from any error
// If the error is not our custom Error type, wraps it
func GetError(err error) *Error {
	if err == nil {
		return nil
	}

	if e, ok := err.(*Error); ok {
		return e
	}

	return Wrap(err, InternalServerError)
}

// Is checks if the error has the given error code
func Is(err error, code ErrorCode) bool {
	if err == nil {
		return false
	}

	if e, ok := err.(*Error); ok {
		return e.Code == code
	}

	return false
}

// getStack captures the stack trace
func getStack(skip int) string {
	const maxDepth = 10
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip+1, pcs[:])

	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var builder strings.Builder

	for {
		frame, more := frames.Next()

		// Skip runtime internal frames
		if strings.Contains(frame.Function, "runtime.") {
			if !more {
				break
			}
			continue
		}

		builder.WriteString(fmt.Sprintf("\n\t%s:%d %s", frame.File, frame.Line, frame.Function))

		if !more {
			break
		}
	}

	return builder.String()
}

// Common error constructors for convenience

// BadRequest creates a bad request error
func BadRequest(msg string) *Error {
	return New(InvalidParams).WithMessage(msg)
}

// NotFoundError creates a not found error
func NotFoundError(resource string) *Error {
	return Newf(NotFound, "%s not found", resource)
}

// UnauthorizedError creates an unauthorized error
func UnauthorizedError(msg string) *Error {
	if msg == "" {
		return New(Unauthorized)
	}
	return New(Unauthorized).WithMessage(msg)
}

// ForbiddenError creates a forbidden error
func ForbiddenError(msg string) *Error {
	if msg == "" {
		return New(Forbidden)
	}
	return New(Forbidden).WithMessage(msg)
}

// InternalError creates an internal server error
func InternalError(err error) *Error {
	if err == nil {
		return New(InternalServerError)
	}
	return Wrap(err, InternalServerError)
}

// ValidationError creates a validation error with details
func ValidationError(field, reason string) *Error {
	return New(ValidationFailed).
		WithDetail("field", field).
		WithDetail("reason", reason)
}
