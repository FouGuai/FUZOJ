// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/submit_service/internal/logic/submit_app"
	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStatusLogic {
	return &GetStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetStatusLogic) GetStatus(req *types.GetStatusRequest) (resp *types.GetStatusResponse, err error) {
	app, err := submit_app.NewSubmitApp(l.svcCtx)
	if err != nil {
		return nil, err
	}
	status, err := app.GetStatus(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return buildStatusResponse(l.ctx, status), nil
}
