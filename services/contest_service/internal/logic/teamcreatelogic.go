// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TeamCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTeamCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TeamCreateLogic {
	return &TeamCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TeamCreateLogic) TeamCreate(req *types.CreateTeamRequest) (resp *types.CreateTeamResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
