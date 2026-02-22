// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/user_service/internal/svc"
	"fuzoj/services/user_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(req *types.LogoutRequest) (resp *types.SuccessResponse, err error) {
	manager := newAuthManager(l.svcCtx)
	if err := manager.Logout(l.ctx, LogoutInput{
		RefreshToken: req.RefreshToken,
	}); err != nil {
		return nil, err
	}
	return buildSuccessResponse(l.ctx, "Logout success"), nil
}
