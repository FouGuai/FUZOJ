// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MyResultLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMyResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MyResultLogic {
	return &MyResultLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MyResultLogic) MyResult(req *types.MyResultRequest) (resp *types.MyResultResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
