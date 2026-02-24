// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HackCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHackCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HackCreateLogic {
	return &HackCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HackCreateLogic) HackCreate(req *types.CreateHackRequest) (resp *types.CreateHackResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
