// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QuitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewQuitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QuitLogic {
	return &QuitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QuitLogic) Quit(req *types.QuitContestRequest) (resp *types.SuccessResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
