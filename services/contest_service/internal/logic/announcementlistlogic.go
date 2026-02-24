// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnnouncementListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnnouncementListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnnouncementListLogic {
	return &AnnouncementListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnnouncementListLogic) AnnouncementList(req *types.ListAnnouncementsRequest) (resp *types.ListAnnouncementsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
