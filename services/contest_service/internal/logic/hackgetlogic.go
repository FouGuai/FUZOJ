// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HackGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHackGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HackGetLogic {
	return &HackGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HackGetLogic) HackGet(req *types.GetHackRequest) (resp *types.GetHackResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
