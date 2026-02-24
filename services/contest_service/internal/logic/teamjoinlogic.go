// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TeamJoinLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTeamJoinLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TeamJoinLogic {
	return &TeamJoinLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TeamJoinLogic) TeamJoin(req *types.JoinTeamRequest) (resp *types.SuccessResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
