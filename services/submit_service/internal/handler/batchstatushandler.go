// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"fuzoj/services/submit_service/internal/logic"
	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func BatchStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BatchStatusRequest
		if err := httpx.Parse(r, &req); err != nil {
			writeError(w, r, badRequestError())
			return
		}

		l := logic.NewBatchStatusLogic(r.Context(), svcCtx)
		resp, err := l.BatchStatus(&req)
		if err != nil {
			writeError(w, r, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
