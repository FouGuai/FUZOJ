// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/logic/problem_app"
	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLatestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLatestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLatestLogic {
	return &GetLatestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetLatestLogic) GetLatest(req *types.GetLatestRequest) (resp *types.LatestMetaResponse, err error) {
	manager := problem_app.NewProblemAppFromContext(l.svcCtx)
	meta, err := manager.GetLatestMeta(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return buildLatestMetaResponse(l.ctx, meta), nil
}
