// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TeamLeaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTeamLeaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TeamLeaveLogic {
	return &TeamLeaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TeamLeaveLogic) TeamLeave(req *types.LeaveTeamRequest) (resp *types.SuccessResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
