package handler

import (
	"net/http"

	"fuzoj/pkg/handlerx"
	"fuzoj/services/rank_service/internal/logic"
	"fuzoj/services/rank_service/internal/svc"
	"fuzoj/services/rank_service/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func LeaderboardHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LeaderboardRequest
		if err := httpx.Parse(r, &req); err != nil {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}
		l := logic.NewLeaderboardLogic(r.Context(), svcCtx)
		resp, err := l.Leaderboard(&req)
		if err != nil {
			handlerx.WriteError(w, r, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
