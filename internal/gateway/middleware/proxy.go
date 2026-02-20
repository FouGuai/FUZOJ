package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/response"

	"github.com/gin-gonic/gin"
)

// ProxyHandler forwards requests to upstream and injects context headers.
func ProxyHandler(proxy *httputil.ReverseProxy, routeName string, timeout time.Duration, stripPrefix string) gin.HandlerFunc {
	if proxy != nil {
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			resp := response.Response{
				Code:    pkgerrors.ServiceUnavailable,
				Message: pkgerrors.ServiceUnavailable.Message(),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(pkgerrors.ServiceUnavailable.HTTPStatus())
			_ = json.NewEncoder(w).Encode(resp)
		}
	}

	return func(c *gin.Context) {
		if proxy == nil {
			response.AbortWithErrorCode(c, pkgerrors.ServiceUnavailable, "upstream proxy unavailable")
			return
		}
		req := c.Request
		if timeout > 0 {
			ctx, cancel := contextWithTimeout(req.Context(), timeout)
			defer cancel()
			req = req.WithContext(ctx)
		}

		if stripPrefix != "" && strings.HasPrefix(req.URL.Path, stripPrefix) {
			path := strings.TrimPrefix(req.URL.Path, stripPrefix)
			if path == "" {
				path = "/"
			}
			req.URL.Path = path
		}

		injectHeaders(c, req, routeName)
		proxy.ServeHTTP(c.Writer, req)
	}
}

func injectHeaders(c *gin.Context, req *http.Request, routeName string) {
	if traceID, ok := c.Get("trace_id"); ok {
		req.Header.Set("X-Trace-Id", toString(traceID))
	}
	if requestID, ok := c.Get("request_id"); ok {
		req.Header.Set("X-Request-Id", toString(requestID))
	}
	if userID, ok := c.Get("user_id"); ok {
		req.Header.Set("X-User-Id", toString(userID))
	}
	if role, ok := c.Get("user_role"); ok {
		req.Header.Set("X-User-Role", toString(role))
	}
	req.Header.Set("X-Route-Name", routeName)
	req.Header.Set("X-Real-IP", c.ClientIP())
}

func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int64:
		return fmt.Sprintf("%d", v)
	case int:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func contextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
