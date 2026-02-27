// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"fuzoj/pkg/handlerx"
	"net/http"

	"fuzoj/services/problem_service/internal/logic"
	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetStatementHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetStatementRequest
		if err := httpx.Parse(r, &req); err != nil {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}

		l := logic.NewGetStatementLogic(r.Context(), svcCtx)
		resp, err := l.GetStatement(&req)
		if err != nil {
			handlerx.WriteError(w, r, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
