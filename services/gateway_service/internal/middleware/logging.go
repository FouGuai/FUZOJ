package middleware

import (
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
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

			logx.WithContext(r.Context()).Infof(
				"request completed method=%s path=%s status=%d latency=%s client_ip=%s",
				r.Method,
				path,
				recorder.status,
				time.Since(start),
				httpx.GetRemoteAddr(r),
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
