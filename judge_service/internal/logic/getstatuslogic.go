// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/judge_service/internal/svc"
	"fuzoj/judge_service/internal/types"

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

func (l *GetStatusLogic) GetStatus(req *types.GetJudgeStatusRequest) (resp *types.JudgeStatusResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
