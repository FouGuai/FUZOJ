// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteLogic {
	return &DeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteLogic) Delete(req *types.DeleteProblemRequest) (resp *types.SuccessResponse, err error) {
	manager := newProblemManagerFromContext(l.svcCtx)
	if err := manager.DeleteProblem(l.ctx, req.Id); err != nil {
		return nil, err
	}
	return buildSuccessResponse(l.ctx, "Success"), nil
}
