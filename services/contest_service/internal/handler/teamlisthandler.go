// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"fuzoj/services/contest_service/internal/logic"
	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TeamListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListTeamsRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewTeamListLogic(r.Context(), svcCtx)
		resp, err := l.TeamList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
