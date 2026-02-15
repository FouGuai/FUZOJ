package gateway_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func performRequest(router http.Handler, method, path string, headers map[string]string) (*httptest.ResponseRecorder, apiResponse, error) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	router.ServeHTTP(rec, req)
	var resp apiResponse
	if rec.Body.Len() > 0 {
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			return rec, resp, err
		}
	}
	return rec, resp, nil
}
