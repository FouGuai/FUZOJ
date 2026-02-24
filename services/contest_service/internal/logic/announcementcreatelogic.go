// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnnouncementCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnnouncementCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnnouncementCreateLogic {
	return &AnnouncementCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnnouncementCreateLogic) AnnouncementCreate(req *types.CreateAnnouncementRequest) (resp *types.CreateAnnouncementResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
