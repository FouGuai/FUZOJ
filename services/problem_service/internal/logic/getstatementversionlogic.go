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

type GetStatementVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetStatementVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStatementVersionLogic {
	return &GetStatementVersionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetStatementVersionLogic) GetStatementVersion(req *types.GetStatementVersionRequest) (resp *types.StatementResponse, err error) {
	ctx, cancel := l.withTimeout()
	if cancel != nil {
		defer cancel()
	}
	problemApp := problem_app.NewProblemAppFromContext(l.svcCtx)
	statement, err := problemApp.GetStatementByVersion(ctx, req.Id, req.Version)
	if err != nil {
		return nil, err
	}
	return buildStatementResponse(ctx, statement), nil
}

func (l *GetStatementVersionLogic) withTimeout() (context.Context, context.CancelFunc) {
	if l.svcCtx == nil || l.svcCtx.Config.Statement.Timeout <= 0 {
		return l.ctx, nil
	}
	ctx, cancel := context.WithTimeout(l.ctx, l.svcCtx.Config.Statement.Timeout)
	return ctx, cancel
}
