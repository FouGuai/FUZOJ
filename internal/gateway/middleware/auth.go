package middleware

import (
	"strings"

	"fuzoj/internal/gateway/service"
	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/response"

	"github.com/gin-gonic/gin"
)

type AuthPolicy struct {
	Mode  string
	Roles []string
}

// AuthMiddleware enforces JWT validation and role checks for protected routes.
func AuthMiddleware(authService *service.AuthService, policy AuthPolicy) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.ToLower(policy.Mode) == "public" {
			c.Next()
			return
		}
		if authService == nil {
			response.AbortWithErrorCode(c, pkgerrors.ServiceUnavailable, "auth service unavailable")
			return
		}

		token := extractBearerToken(c.GetHeader("Authorization"))
		info, err := authService.Authenticate(c.Request.Context(), token)
		if err != nil {
			response.AbortWithError(c, err)
			return
		}

		if len(policy.Roles) > 0 && !hasRole(info.Role, policy.Roles) {
			response.AbortWithErrorCode(c, pkgerrors.Forbidden, "insufficient role")
			return
		}

		c.Set("user_id", info.ID)
		c.Set("user_role", info.Role)
		c.Next()
	}
}

func extractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func hasRole(role string, allowed []string) bool {
	for _, item := range allowed {
		if strings.EqualFold(role, item) {
			return true
		}
	}
	return false
}
