package middleware

import (
	"net/http"
	"strings"
)

type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           string
}

// CORSMiddleware applies basic CORS headers for browser clients.
func CORSMiddleware(cfg CORSConfig) func(http.HandlerFunc) http.HandlerFunc {
	if !cfg.Enabled {
		return func(next http.HandlerFunc) http.HandlerFunc { return next }
	}
	allowedOrigins := strings.Join(cfg.AllowedOrigins, ",")
	allowedMethods := strings.Join(cfg.AllowedMethods, ",")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ",")
	exposedHeaders := strings.Join(cfg.ExposedHeaders, ",")

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next(w, r)
				return
			}

			if !isOriginAllowed(origin, cfg.AllowedOrigins) {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next(w, r)
				return
			}

			if allowedOrigins == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			if allowedMethods != "" {
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			}
			if allowedHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			}
			if exposedHeaders != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
			}
			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if cfg.MaxAge != "" {
				w.Header().Set("Access-Control-Max-Age", cfg.MaxAge)
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next(w, r)
		}
	}
}

func isOriginAllowed(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, item := range allowed {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if item == "*" || strings.EqualFold(item, origin) {
			return true
		}
	}
	return false
}
