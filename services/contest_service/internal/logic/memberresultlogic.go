// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MemberResultLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMemberResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MemberResultLogic {
	return &MemberResultLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MemberResultLogic) MemberResult(req *types.MemberResultRequest) (resp *types.MyResultResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
