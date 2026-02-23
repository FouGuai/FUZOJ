// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"context"
	"net/http"

	"fuzoj/pkg/utils/contextkey"
	"fuzoj/services/submit_service/internal/logic"
	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateSubmissionRequest
		if err := httpx.Parse(r, &req); err != nil {
			writeError(w, r, badRequestError())
			return
		}

		ctx := context.WithValue(r.Context(), contextkey.ClientIP, httpx.GetRemoteAddr(r))
		l := logic.NewCreateLogic(ctx, svcCtx)
		resp, err := l.Create(&req)
		if err != nil {
			writeError(w, r, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
