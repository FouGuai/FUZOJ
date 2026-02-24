// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCloseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseLogic {
	return &CloseLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CloseLogic) Close(req *types.GetContestRequest) (resp *types.SuccessResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
