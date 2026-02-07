package main

import (
	"context"
	"fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"
	"fuzoj/pkg/utils/response"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ==================== Example 1: Basic Error Handling ====================

// UserService demonstrates error handling in service layer
type UserService struct{}

func (s *UserService) GetUser(ctx context.Context, userID int64) (*User, error) {
	// Scenario 1: User not found
	if userID == 0 {
		return nil, errors.New(errors.UserNotFound)
	}

	// Scenario 2: User not found with custom message
	if userID < 0 {
		return nil, errors.Newf(errors.UserNotFound, "user with id %d does not exist", userID)
	}

	// Scenario 3: Database error with wrapping
	// err := db.Query(...)
	// if err != nil {
	//     return nil, errors.Wrap(err, errors.DatabaseError)
	// }

	// Scenario 4: Add context details
	user := &User{ID: userID, Username: "testuser"}
	logger.Info(ctx, "user retrieved successfully",
		zap.Int64("user_id", userID),
		zap.String("username", user.Username),
	)

	return user, nil
}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) error {
	// Scenario 5: Validation error with details
	if req.Username == "" {
		return errors.New(errors.RequiredFieldEmpty).
			WithDetail("field", "username").
			WithDetail("requirement", "must not be empty")
	}

	// Scenario 6: Business rule violation
	if len(req.Password) < 8 {
		return errors.New(errors.PasswordTooWeak).
			WithMessage("password must be at least 8 characters").
			WithDetail("min_length", 8).
			WithDetail("provided_length", len(req.Password))
	}

	// Scenario 7: Username already exists
	// exists, err := s.repo.UsernameExists(ctx, req.Username)
	// if exists {
	//     return errors.New(errors.UsernameAlreadyExists).
	//         WithDetail("username", req.Username)
	// }

	logger.Info(ctx, "user created successfully",
		zap.String("username", req.Username),
	)

	return nil
}

// ==================== Example 2: Handler Layer ====================

type UserHandler struct {
	service *UserService
}

// GetUserHandler demonstrates handler error handling
func (h *UserHandler) GetUserHandler(c *gin.Context) {
	// Extract user ID from path
	userID := int64(1) // In real code: parse from c.Param("id")

	// Call service
	user, err := h.service.GetUser(c.Request.Context(), userID)
	if err != nil {
		// Auto-detect error code and send appropriate response
		response.Error(c, err)
		return
	}

	// Success response
	response.Success(c, user)
}

// CreateUserHandler demonstrates validation and error handling
func (h *UserHandler) CreateUserHandler(c *gin.Context) {
	var req CreateUserRequest

	// Bind JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	// Call service
	if err := h.service.CreateUser(c.Request.Context(), &req); err != nil {
		// Response package will auto-detect error code and HTTP status
		response.Error(c, err)
		return
	}

	response.SuccessWithMessage(c, "user created successfully", nil)
}

// ==================== Example 3: Logger Usage ====================

func LoggerExamples(ctx context.Context) {
	// Basic logging
	logger.Info(ctx, "user logged in")

	// Logging with fields
	logger.Info(ctx, "processing submission",
		zap.Int64("submission_id", 12345),
		zap.String("language", "cpp"),
		zap.Int("problem_id", 100),
	)

	// Debug logging (won't show in production)
	logger.Debug(ctx, "cache hit",
		zap.String("key", "user:123"),
		zap.Duration("latency", 5000),
	)

	// Warning
	logger.Warn(ctx, "cache miss, falling back to database",
		zap.String("key", "user:123"),
	)

	// Error logging
	logger.Error(ctx, "failed to send email",
		zap.String("recipient", "user@example.com"),
		zap.Error(errors.New(errors.InternalServerError)),
	)

	// Formatted logging
	logger.Infof(ctx, "user %s submitted problem %d", "alice", 100)
	logger.Errorf(ctx, "failed to connect to redis: %v", "connection refused")
}

// ==================== Example 4: Repository Layer ====================

type UserRepository struct{}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	// Log the database query
	logger.Debug(ctx, "querying user by id", zap.Int64("user_id", id))

	// Simulate database error
	// err := db.First(&user, id).Error
	// if err == gorm.ErrRecordNotFound {
	//     return nil, errors.New(errors.UserNotFound).
	//         WithDetail("user_id", id)
	// }
	// if err != nil {
	//     logger.Error(ctx, "database query failed",
	//         zap.Error(err),
	//         zap.Int64("user_id", id),
	//     )
	//     return nil, errors.Wrap(err, errors.DatabaseError)
	// }

	user := &User{ID: id, Username: "testuser"}
	logger.Debug(ctx, "user found", zap.Any("user", user))

	return user, nil
}

// ==================== Example 5: Adding New Error Codes ====================

// Step 1: Add to code.go
// const (
//     // Add your new error code in the appropriate range
//     CustomFeatureNotEnabled ErrorCode = 16200
// )

// Step 2: Add to errorMessages map in code.go
// var errorMessages = map[ErrorCode]string{
//     ...
//     CustomFeatureNotEnabled: "This feature is not enabled for your account",
// }

// Step 3: Use it in your code
func UseNewErrorCode(ctx context.Context) error {
	// Direct usage
	// return errors.New(errors.CustomFeatureNotEnabled)

	// With custom message
	// return errors.Newf(errors.CustomFeatureNotEnabled,
	//     "feature %s requires premium subscription", "advanced_analytics")

	// With details
	// return errors.New(errors.CustomFeatureNotEnabled).
	//     WithDetail("feature", "advanced_analytics").
	//     WithDetail("required_plan", "premium")

	return nil
}

// ==================== Example 6: Error Checking ====================

func ErrorCheckingExample(ctx context.Context) {
	err := errors.New(errors.UserNotFound)

	// Check if error is specific code
	if errors.Is(err, errors.UserNotFound) {
		logger.Info(ctx, "user not found, creating new user")
	}

	// Extract error code
	code := errors.GetCode(err)
	logger.Info(ctx, "error occurred", zap.Int("code", int(code)))

	// Get HTTP status
	status := code.HTTPStatus()
	logger.Info(ctx, "http status", zap.Int("status", status))
}

// ==================== Example 7: Pagination Response ====================

func PaginationExample(c *gin.Context) {
	// Simulate fetching users
	users := []User{
		{ID: 1, Username: "alice"},
		{ID: 2, Username: "bob"},
	}

	total := int64(100)
	page := 1
	pageSize := 10

	response.SuccessWithPagination(c, users, total, page, pageSize)
	// Output:
	// {
	//   "code": 10000,
	//   "message": "Success",
	//   "data": {
	//     "items": [...],
	//     "total": 100,
	//     "page": 1,
	//     "page_size": 10,
	//     "total_pages": 10
	//   }
	// }
}

// ==================== Models ====================

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// ==================== Main Function ====================

func main() {
	// Initialize logger
	loggerConfig := logger.Config{
		Level:      "debug",   // debug, info, warn, error
		Format:     "console", // json, console
		OutputPath: "stdout",  // stdout or file path
		ErrorPath:  "stderr",  // stderr or file path
	}

	if err := logger.Init(loggerConfig); err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Initialize Gin
	r := gin.Default()

	// Add trace ID middleware
	r.Use(func(c *gin.Context) {
		// Generate or extract trace ID
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = "trace-" + c.GetString("request_id")
		}
		c.Set("trace_id", traceID)

		// Add to context
		ctx := context.WithValue(c.Request.Context(), "trace_id", traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	})

	// Setup routes
	service := &UserService{}
	handler := &UserHandler{service: service}

	r.GET("/users/:id", handler.GetUserHandler)
	r.POST("/users", handler.CreateUserHandler)

	// Start server
	logger.Info(context.Background(), "server starting", zap.Int("port", 8080))
	if err := r.Run(":8080"); err != nil {
		logger.Fatal(context.Background(), "failed to start server", zap.Error(err))
	}
}

// ==================== Example API Responses ====================

// Success Response:
// {
//   "code": 10000,
//   "message": "Success",
//   "data": {
//     "id": 1,
//     "username": "alice",
//     "email": "alice@example.com"
//   },
//   "trace_id": "trace-abc123"
// }

// Error Response:
// {
//   "code": 11001,
//   "message": "User not found",
//   "details": {
//     "user_id": 999
//   },
//   "trace_id": "trace-abc123"
// }

// Validation Error Response:
// {
//   "code": 10303,
//   "message": "Required field is empty",
//   "details": {
//     "field": "username",
//     "requirement": "must not be empty"
//   },
//   "trace_id": "trace-abc123"
// }
