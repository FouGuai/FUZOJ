package gateway_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
