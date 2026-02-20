# Error Code & Logger System Guide

## Overview

This is a production-ready error handling and logging system designed for FuzOJ. It provides:

- ✅ **Structured Error Codes** - Organized by module (10000-16999)
- ✅ **Automatic HTTP Status Mapping** - Error codes map to appropriate HTTP status
- ✅ **Context Tracing** - Trace ID support for distributed tracing
- ✅ **Structured Logging** - Based on uber-go/zap for high performance
- ✅ **Easy to Extend** - Add new error codes without breaking existing code
- ✅ **Developer Friendly** - Simple API, one-line error creation

---

## Quick Start

### 1. Initialize Logger

```go
import "fuzoj/pkg/utils/logger"

func main() {
    cfg := logger.Config{
        Level:      "info",       // debug, info, warn, error
        Format:     "json",       // json or console
        OutputPath: "stdout",     // stdout or file path
        ErrorPath:  "stderr",     // stderr or file path
    }
    
    if err := logger.Init(cfg); err != nil {
        panic(err)
    }
    defer logger.Sync()
}
```

### 2. Use Error Codes

```go
import (
    "fuzoj/pkg/errors"
    "fuzoj/pkg/utils/logger"
)

func GetUser(ctx context.Context, userID int64) (*User, error) {
    // Simple error
    if userID == 0 {
        return nil, errors.New(errors.UserNotFound)
    }
    
    // Error with custom message
    if userID < 0 {
        return nil, errors.Newf(errors.UserNotFound, 
            "user %d does not exist", userID)
    }
    
    // Wrap existing error
    user, err := db.GetUser(userID)
    if err != nil {
        return nil, errors.Wrap(err, errors.DatabaseError)
    }
    
    // Error with details
    if !user.IsActive {
        return nil, errors.New(errors.AccountSuspended).
            WithDetail("user_id", userID).
            WithDetail("suspended_at", user.SuspendedAt)
    }
    
    return user, nil
}
```

### 3. Log Messages

```go
import (
    "fuzoj/pkg/utils/logger"
    "go.uber.org/zap"
)

// Simple logging
logger.Info(ctx, "user logged in")

// With fields
logger.Info(ctx, "submission created",
    zap.Int64("submission_id", 12345),
    zap.String("language", "cpp"),
)

// Error logging
logger.Error(ctx, "database connection failed",
    zap.Error(err),
    zap.String("host", "localhost"),
)

// Debug logging (not shown in production)
logger.Debug(ctx, "cache hit", zap.String("key", "user:123"))
```

### 4. Handler Response

```go
import (
    "fuzoj/pkg/utils/response"
    "github.com/gin-gonic/gin"
)

func GetUserHandler(c *gin.Context) {
    user, err := service.GetUser(c.Request.Context(), userID)
    if err != nil {
        // Auto-detect error code and HTTP status
        response.Error(c, err)
        return
    }
    
    // Success response
    response.Success(c, user)
}

// Output on error:
// HTTP 404
// {
//   "code": 11001,
//   "message": "User not found",
//   "trace_id": "abc123"
// }
```

---

## Error Code Ranges

| Range | Module | Examples |
|-------|--------|----------|
| 10000-10999 | System & Common | Database, Cache, Validation |
| 11000-11999 | User Module | Authentication, Registration |
| 12000-12999 | Problem Module | Problem CRUD, Test Cases |
| 13000-13999 | Submission & Judge | Compilation, Runtime Errors |
| 14000-14999 | Contest Module | Registration, Ranking |
| 15000-15999 | Discussion | Comments, Posts |
| 16000-16999 | Admin & Permission | RBAC, Admin Operations |

---

## Adding New Error Codes

### Step 1: Define Error Code

Edit `pkg/errors/code.go`:

```go
const (
    // Add in appropriate range
    FeatureNotAvailable ErrorCode = 16200
)
```

### Step 2: Add Message

In the same file, add to `errorMessages` map:

```go
var errorMessages = map[ErrorCode]string{
    // ...
    FeatureNotAvailable: "This feature is not available",
}
```

### Step 3: Use It

```go
func CheckFeature(ctx context.Context, feature string) error {
    if !isEnabled(feature) {
        return errors.New(errors.FeatureNotAvailable).
            WithDetail("feature", feature).
            WithDetail("available_in", "premium plan")
    }
    return nil
}
```

---

## API Response Format

### Success Response

```json
{
  "code": 10000,
  "message": "Success",
  "data": {
    "id": 1,
    "username": "alice"
  },
  "trace_id": "trace-abc123"
}
```

### Error Response

```json
{
  "code": 11001,
  "message": "User not found",
  "details": {
    "user_id": 999
  },
  "trace_id": "trace-abc123"
}
```

### Pagination Response

```json
{
  "code": 10000,
  "message": "Success",
  "data": {
    "items": [...],
    "total": 100,
    "page": 1,
    "page_size": 10,
    "total_pages": 10
  },
  "trace_id": "trace-abc123"
}
```

---

## Best Practices

### ✅ DO

```go
// Return specific error codes
return errors.New(errors.UserNotFound)

// Add context with details
return errors.New(errors.ValidationFailed).
    WithDetail("field", "email").
    WithDetail("reason", "invalid format")

// Wrap database errors
if err != nil {
    return errors.Wrap(err, errors.DatabaseError)
}

// Log with structured fields
logger.Info(ctx, "user created", 
    zap.String("username", user.Username),
    zap.Int64("user_id", user.ID),
)
```

### ❌ DON'T

```go
// Don't return raw errors
return err // ❌

// Don't use generic errors
return errors.New("something went wrong") // ❌

// Don't log unstructured data
logger.Info(ctx, fmt.Sprintf("user %v created", user)) // ❌

// Don't use wrong error code
return errors.New(errors.InternalServerError) // ❌ (for validation)
```

---

## Error Code to HTTP Status Mapping

| Error Code Range | HTTP Status | Example |
|-----------------|-------------|---------|
| 10000 | 200 OK | Success |
| 10002, 10300-10399 | 400 Bad Request | Invalid Parameters |
| 11000-11099 | 401 Unauthorized | Invalid Credentials |
| 10005, 16000-16099 | 403 Forbidden | Permission Denied |
| 10003, XX001 | 404 Not Found | Resource Not Found |
| 10006, 13004 | 429 Too Many Requests | Rate Limited |
| Others | 500 Internal Error | Server Error |

---

## Logger Configuration

### Development Environment

```go
logger.Config{
    Level:      "debug",     // Show all logs
    Format:     "console",   // Readable format
    OutputPath: "stdout",
}
```

### Production Environment

```go
logger.Config{
    Level:      "info",                  // Hide debug logs
    Format:     "json",                  // Structured format
    OutputPath: "/var/log/fuzoj/app.log", // File output
    ErrorPath:  "/var/log/fuzoj/error.log",
}
```

---

## Context Tracing

### Add Trace ID Middleware

```go
r.Use(func(c *gin.Context) {
    traceID := uuid.New().String()
    c.Set("trace_id", traceID)
    
    ctx := context.WithValue(c.Request.Context(), contextkey.TraceID, traceID)
    c.Request = c.Request.WithContext(ctx)
    
    c.Next()
})
```

### Logger Automatically Extracts Context

```go
// Logger will automatically include trace_id
logger.Info(ctx, "processing request") 

// Output:
// {"level":"info","time":"2026-02-01T10:00:00Z","trace_id":"abc-123","msg":"processing request"}
```

---

## Performance Considerations

- ✅ **Structured logging is fast** - zap is one of the fastest Go loggers
- ✅ **Error codes are compile-time constants** - No runtime overhead
- ✅ **Lazy evaluation** - Log fields are only evaluated if level is enabled
- ✅ **Async logging** - Can enable async mode for even higher throughput

```go
// Debug logs are skipped entirely in production (level=info)
logger.Debug(ctx, "expensive operation", 
    zap.Any("data", expensiveFunction()), // Not called if debug disabled
)
```

---

## Common Error Scenarios

### Database Errors

```go
user, err := db.First(&user, id)
if err == gorm.ErrRecordNotFound {
    return nil, errors.New(errors.UserNotFound)
}
if err != nil {
    return nil, errors.Wrap(err, errors.DatabaseError)
}
```

### Validation Errors

```go
if req.Email == "" {
    return errors.New(errors.RequiredFieldEmpty).
        WithDetail("field", "email")
}
if !isValidEmail(req.Email) {
    return errors.New(errors.InvalidEmail).
        WithDetail("email", req.Email)
}
```

### Permission Errors

```go
if user.Role != "admin" {
    return errors.New(errors.PermissionDenied).
        WithDetail("required_role", "admin").
        WithDetail("user_role", user.Role)
}
```

---

## Testing

### Mock Errors

```go
func TestGetUser(t *testing.T) {
    service := NewUserService(mockRepo)
    
    _, err := service.GetUser(ctx, 999)
    
    // Check error code
    assert.True(t, errors.Is(err, errors.UserNotFound))
    
    // Check HTTP status
    assert.Equal(t, 404, errors.GetCode(err).HTTPStatus())
}
```

---

## Summary

- 🎯 **Use specific error codes** for better debugging
- 📊 **Structure your logs** with zap fields
- 🔍 **Always include trace_id** for distributed tracing
- 📝 **Add details to errors** for context
- ⚡ **Wrap database errors** to categorize them
- 🚀 **Keep extending** - Add new codes as needed

For more examples, see `examples/error_logger_usage.go`
