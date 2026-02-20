package common_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	commonmw "fuzoj/internal/common/http/middleware"
	"fuzoj/pkg/utils/contextkey"

	"github.com/gin-gonic/gin"
)

type traceResponse struct {
	TraceID      string `json:"trace_id"`
	RequestID    string `json:"request_id"`
	UserID       string `json:"user_id"`
	CtxTraceID   string `json:"ctx_trace_id"`
	CtxRequestID string `json:"ctx_request_id"`
	CtxUserID    string `json:"ctx_user_id"`
}

func TestTraceContextMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(commonmw.TraceContextMiddleware())
	router.GET("/trace", func(c *gin.Context) {
		traceID, _ := c.Get("trace_id")
		requestID, _ := c.Get("request_id")
		userID, _ := c.Get("user_id")
		ctx := c.Request.Context()
		c.JSON(http.StatusOK, traceResponse{
			TraceID:      toString(traceID),
			RequestID:    toString(requestID),
			UserID:       toString(userID),
			CtxTraceID:   toString(ctx.Value(contextkey.TraceID)),
			CtxRequestID: toString(ctx.Value(contextkey.RequestID)),
			CtxUserID:    toString(ctx.Value(contextkey.UserID)),
		})
	})

	cases := []struct {
		name              string
		headers           map[string]string
		expectTraceID     bool
		expectRequestID   bool
		expectedTraceID   string
		expectedRequestID string
		expectedUserID    string
	}{
		{
			name:            "generate trace and request id",
			headers:         nil,
			expectTraceID:   true,
			expectRequestID: true,
		},
		{
			name: "preserve trace request and user id",
			headers: map[string]string{
				"X-Trace-Id":   "trace-123",
				"X-Request-Id": "req-123",
				"X-User-Id":    "42",
			},
			expectTraceID:     true,
			expectRequestID:   true,
			expectedTraceID:   "trace-123",
			expectedRequestID: "req-123",
			expectedUserID:    "42",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/trace", nil)
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}
			router.ServeHTTP(rec, req)

			var resp traceResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response failed: %v", err)
			}

			if tc.expectTraceID && resp.TraceID == "" {
				t.Fatalf("expected trace id in response")
			}
			if tc.expectRequestID && resp.RequestID == "" {
				t.Fatalf("expected request id in response")
			}
			if tc.expectedTraceID != "" && resp.TraceID != tc.expectedTraceID {
				t.Fatalf("expected trace id %s, got %s", tc.expectedTraceID, resp.TraceID)
			}
			if tc.expectedRequestID != "" && resp.RequestID != tc.expectedRequestID {
				t.Fatalf("expected request id %s, got %s", tc.expectedRequestID, resp.RequestID)
			}
			if tc.expectedUserID != "" && resp.UserID != tc.expectedUserID {
				t.Fatalf("expected user id %s, got %s", tc.expectedUserID, resp.UserID)
			}
			if tc.expectTraceID && resp.CtxTraceID == "" {
				t.Fatalf("expected trace id in request context")
			}
			if tc.expectRequestID && resp.CtxRequestID == "" {
				t.Fatalf("expected request id in request context")
			}
			if tc.expectedTraceID != "" && resp.CtxTraceID != tc.expectedTraceID {
				t.Fatalf("expected trace id %s in request context, got %s", tc.expectedTraceID, resp.CtxTraceID)
			}
			if tc.expectedRequestID != "" && resp.CtxRequestID != tc.expectedRequestID {
				t.Fatalf("expected request id %s in request context, got %s", tc.expectedRequestID, resp.CtxRequestID)
			}
			if tc.expectedUserID != "" && resp.CtxUserID != tc.expectedUserID {
				t.Fatalf("expected user id %s in request context, got %s", tc.expectedUserID, resp.CtxUserID)
			}

			if tc.expectTraceID && rec.Header().Get("X-Trace-Id") == "" {
				t.Fatalf("expected trace id header")
			}
			if tc.expectRequestID && rec.Header().Get("X-Request-Id") == "" {
				t.Fatalf("expected request id header")
			}
			if tc.expectedTraceID != "" && rec.Header().Get("X-Trace-Id") != tc.expectedTraceID {
				t.Fatalf("expected trace id header")
			}
			if tc.expectedRequestID != "" && rec.Header().Get("X-Request-Id") != tc.expectedRequestID {
				t.Fatalf("expected request id header")
			}
			if tc.expectedUserID != "" && rec.Header().Get("X-User-Id") != tc.expectedUserID {
				t.Fatalf("expected user id header")
			}
		})
	}
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}
