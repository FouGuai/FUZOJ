// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TeamListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTeamListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TeamListLogic {
	return &TeamListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TeamListLogic) TeamList(req *types.ListTeamsRequest) (resp *types.ListTeamsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
