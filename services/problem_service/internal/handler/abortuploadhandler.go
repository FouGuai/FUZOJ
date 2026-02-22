// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"fuzoj/services/problem_service/internal/logic"
	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AbortUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AbortUploadRequest
		if err := httpx.Parse(r, &req); err != nil {
			writeError(w, r, badRequestError())
			return
		}

		l := logic.NewAbortUploadLogic(r.Context(), svcCtx)
		resp, err := l.AbortUpload(&req)
		if err != nil {
			writeError(w, r, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
