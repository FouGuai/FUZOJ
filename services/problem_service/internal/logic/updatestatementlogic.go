// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/logic/problem_app"
	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateStatementLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateStatementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateStatementLogic {
	return &UpdateStatementLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateStatementLogic) UpdateStatement(req *types.UpdateStatementRequest) (resp *types.SuccessResponse, err error) {
	ctx, cancel := l.withTimeout()
	if cancel != nil {
		defer cancel()
	}
	problemApp := problem_app.NewProblemAppFromContext(l.svcCtx)
	if err := problemApp.UpdateStatement(ctx, req.Id, req.Version, req.StatementMd); err != nil {
		return nil, err
	}
	return buildSuccessResponse(ctx, "Success"), nil
}

func (l *UpdateStatementLogic) withTimeout() (context.Context, context.CancelFunc) {
	if l.svcCtx == nil || l.svcCtx.Config.Statement.Timeout <= 0 {
		return l.ctx, nil
	}
	ctx, cancel := context.WithTimeout(l.ctx, l.svcCtx.Config.Statement.Timeout)
	return ctx, cancel
}
