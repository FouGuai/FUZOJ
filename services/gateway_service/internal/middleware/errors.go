package middleware

import (
	"net/http"

	"fuzoj/services/gateway_service/internal/response"
)

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	response.WriteError(w, r, err)
}
