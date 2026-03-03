package handler

import (
	"net/http"

	"fuzoj/services/rank_service/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/:id/leaderboard",
				Handler: LeaderboardHandler(serverCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/:id/leaderboard/ws",
				Handler: LeaderboardWSHandler(serverCtx),
			},
		},
		rest.WithPrefix("/api/v1/contests"),
	)
}
