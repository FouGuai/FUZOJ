package handler

import (
	"net/http"

	"fuzoj/services/status_sse_service/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes(
		[]rest.Route{
			{
				Method:  http.MethodGet,
				Path:    "/submissions/:id/events",
				Handler: StatusEventsHandler(serverCtx),
			},
		},
		rest.WithPrefix("/api/v1/status"),
	)
}
