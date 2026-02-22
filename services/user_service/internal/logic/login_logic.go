// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"strings"

	"fuzoj/services/user_service/internal/svc"
	"fuzoj/services/user_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginRequest) (resp *types.AuthResponse, err error) {
	manager := newAuthManager(l.svcCtx)
	result, err := manager.Login(l.ctx, LoginInput{
		Username:   strings.TrimSpace(req.Username),
		Password:   req.Password,
		IP:         req.IP,
		DeviceInfo: req.DeviceInfo,
	})
	if err != nil {
		return nil, err
	}
	return buildAuthResponse(l.ctx, result), nil
}
