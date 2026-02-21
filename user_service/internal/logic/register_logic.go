// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"strings"

	"fuzoj/user_service/internal/service"
	"fuzoj/user_service/internal/svc"
	"fuzoj/user_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterRequest) (resp *types.AuthResponse, err error) {
	result, err := l.svcCtx.AuthService.Register(l.ctx, service.RegisterInput{
		Username: strings.TrimSpace(req.Username),
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	return buildAuthResponse(l.ctx, result), nil
}
