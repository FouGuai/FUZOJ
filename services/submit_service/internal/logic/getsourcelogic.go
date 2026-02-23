// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSourceLogic {
	return &GetSourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSourceLogic) GetSource(req *types.GetSourceRequest) (resp *types.GetSourceResponse, err error) {
	app, err := NewSubmitApp(l.svcCtx)
	if err != nil {
		return nil, err
	}
	submission, err := app.GetSource(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return buildSourceResponse(l.ctx, submission), nil
}
