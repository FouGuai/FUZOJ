package gateway_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
	closeCh chan bool
}

func newCloseNotifyRecorder() *closeNotifyRecorder {
	return &closeNotifyRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeCh:          make(chan bool, 1),
	}
}

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	return r.closeCh
}

func performRequest(router http.Handler, method, path string, headers map[string]string) (*closeNotifyRecorder, apiResponse, error) {
	rec := newCloseNotifyRecorder()
	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	router.ServeHTTP(rec, req)
	var resp apiResponse
	body := rec.Body.Bytes()
	if len(body) > 0 && looksLikeJSON(body) {
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			return rec, resp, err
		}
	}
	return rec, resp, nil
}

func looksLikeJSON(body []byte) bool {
	body = bytes.TrimSpace(body)
	return len(body) > 0 && body[0] == '{'
}

func applyMiddleware(handler http.HandlerFunc, middlewares ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

func newAccessToken(t testingT, secret, issuer string, userID int64, role string, exp time.Duration) string {
	claims := jwt.MapClaims{
		"role": role,
		"typ":  "access",
		"sub":  fmt.Sprintf("%d", userID),
		"iss":  issuer,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(exp).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}
	return raw
}

func pickFreePort(t testingT) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

type testingT interface {
	Fatalf(format string, args ...interface{})
}
