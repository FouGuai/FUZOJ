package controller

import (
	"strings"
	"time"

	"fuzoj/internal/user/service"
	"fuzoj/pkg/utils/response"

	"github.com/gin-gonic/gin"
)

// AuthController handles auth-related HTTP endpoints.
type AuthController struct {
	authService *service.AuthService
}

// NewAuthController creates a new AuthController.
func NewAuthController(authService *service.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

// Register handles user registration.
func (h *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	result, err := h.authService.Register(c.Request.Context(), service.RegisterInput{
		Username: strings.TrimSpace(req.Username),
		Password: req.Password,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, toAuthResponse(result))
}

// Login handles user login.
func (h *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	result, err := h.authService.Login(c.Request.Context(), service.LoginInput{
		Username:   strings.TrimSpace(req.Username),
		Password:   req.Password,
		IP:         c.ClientIP(),
		DeviceInfo: c.GetHeader("User-Agent"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, toAuthResponse(result))
}

// Refresh handles token refresh.
func (h *AuthController) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	result, err := h.authService.Refresh(c.Request.Context(), service.RefreshInput{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, toAuthResponse(result))
}

// Logout handles refresh token revocation.
func (h *AuthController) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if err := h.authService.Logout(c.Request.Context(), service.LogoutInput{
		RefreshToken: req.RefreshToken,
	}); err != nil {
		response.Error(c, err)
		return
	}

	response.SuccessWithMessage(c, "Logout success", nil)
}

// RegisterRequest defines registration payload.
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest defines login payload.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest defines refresh payload.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest defines logout payload.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// AuthResponse defines auth response payload.
type AuthResponse struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	User             UserInfo  `json:"user"`
}

// UserInfo defines basic user info payload.
type UserInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func toAuthResponse(result service.AuthResult) AuthResponse {
	return AuthResponse{
		AccessToken:      result.AccessToken,
		RefreshToken:     result.RefreshToken,
		AccessExpiresAt:  result.AccessExpiresAt,
		RefreshExpiresAt: result.RefreshExpiresAt,
		User: UserInfo{
			ID:       result.User.ID,
			Username: result.User.Username,
			Role:     string(result.User.Role),
		},
	}
}
