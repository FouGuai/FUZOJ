// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"fuzoj/pkg/handlerx"
	"net/http"

	"fuzoj/services/user_service/internal/logic"
	"fuzoj/services/user_service/internal/svc"
	"fuzoj/services/user_service/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func LogoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LogoutRequest
		if err := httpx.Parse(r, &req); err != nil {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}
		if req.RefreshToken == "" {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}

		l := logic.NewLogoutLogic(r.Context(), svcCtx)
		resp, err := l.Logout(&req)
		if err != nil {
			handlerx.WriteError(w, r, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
