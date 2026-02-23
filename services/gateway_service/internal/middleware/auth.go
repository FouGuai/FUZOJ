package middleware

import (
	"net/http"
	"strings"

	"fuzoj/pkg/errors"
	"fuzoj/services/gateway_service/internal/service"
)

type AuthPolicy struct {
	Mode  string
	Roles []string
}

// AuthMiddleware enforces JWT validation and role checks for protected routes.
func AuthMiddleware(authService *service.AuthService) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			policy := getRoutePolicy(r.Context())
			if strings.ToLower(policy.Auth.Mode) == "public" {
				next(w, r)
				return
			}
			if authService == nil {
				WriteError(w, r, errors.New(errors.ServiceUnavailable).WithMessage("auth service unavailable"))
				return
			}

			token := extractBearerToken(r.Header.Get("Authorization"))
			info, err := authService.Authenticate(r.Context(), token)
			if err != nil {
				WriteError(w, r, err)
				return
			}

			if len(policy.Auth.Roles) > 0 && !hasRole(info.Role, policy.Auth.Roles) {
				WriteError(w, r, errors.New(errors.Forbidden).WithMessage("insufficient role"))
				return
			}

			ctx := withUserInfo(r.Context(), info.ID, info.Role)
			r = r.WithContext(ctx)
			next(w, r)
		}
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
