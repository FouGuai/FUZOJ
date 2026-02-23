package middleware

import (
	"net/http"
	"time"

	"fuzoj/pkg/utils/logger"

	"github.com/zeromicro/go-zero/rest/httpx"
	"go.uber.org/zap"
)

// RequestLogger logs request summaries after handling.
func RequestLogger() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next(recorder, r)

			path := r.URL.Path
			if routePath, ok := getRoutePath(r.Context()); ok {
				path = routePath
			}

			logger.Info(
				r.Context(),
				"request completed",
				zap.String("method", r.Method),
				zap.String("path", path),
				zap.Int("status", recorder.status),
				zap.Duration("latency", time.Since(start)),
				zap.String("client_ip", httpx.GetRemoteAddr(r)),
			)
		}
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
