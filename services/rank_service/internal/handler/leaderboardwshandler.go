package handler

import (
	"context"
	"net/http"

	"fuzoj/pkg/handlerx"
	"fuzoj/services/rank_service/internal/logic"
	"fuzoj/services/rank_service/internal/svc"
	"fuzoj/services/rank_service/internal/types"
	"fuzoj/services/rank_service/internal/ws"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func LeaderboardWSHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LeaderboardRequest
		if err := httpx.Parse(r, &req); err != nil {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}
		if req.Id == "" {
			handlerx.WriteError(w, r, handlerx.BadRequestError())
			return
		}
		mode, err := logic.NormalizeLeaderboardMode(req.Mode)
		if err != nil {
			handlerx.WriteError(w, r, err)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("upgrade websocket failed: %v", err)
			return
		}
		if svcCtx.Hub == nil {
			_ = conn.Close()
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		svcCtx.Hub.Subscribe(ctx, req.Id, req.Page, req.PageSize, mode, ws.NewSender(conn))
	}
}
