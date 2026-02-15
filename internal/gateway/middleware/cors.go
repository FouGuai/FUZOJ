package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
func CORSMiddleware(cfg CORSConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	allowedOrigins := strings.Join(cfg.AllowedOrigins, ",")
	allowedMethods := strings.Join(cfg.AllowedMethods, ",")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ",")
	exposedHeaders := strings.Join(cfg.ExposedHeaders, ",")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		if !isOriginAllowed(origin, cfg.AllowedOrigins) {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		if allowedOrigins == "*" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}
		if allowedMethods != "" {
			c.Writer.Header().Set("Access-Control-Allow-Methods", allowedMethods)
		}
		if allowedHeaders != "" {
			c.Writer.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		}
		if exposedHeaders != "" {
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
		}
		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if cfg.MaxAge != "" {
			c.Writer.Header().Set("Access-Control-Max-Age", cfg.MaxAge)
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
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
