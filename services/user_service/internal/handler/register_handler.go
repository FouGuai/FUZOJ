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

func RegisterHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RegisterRequest
		if err := httpx.Parse(r, &req); err != nil {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}
		if req.Username == "" || req.Password == "" {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}

		l := logic.NewRegisterLogic(r.Context(), svcCtx)
		resp, err := l.Register(&req)
		if err != nil {
			handlerx.WriteError(w, r, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
