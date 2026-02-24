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

type GetStatementLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetStatementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStatementLogic {
	return &GetStatementLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetStatementLogic) GetStatement(req *types.GetStatementRequest) (resp *types.StatementResponse, err error) {
	ctx, cancel := l.withTimeout()
	if cancel != nil {
		defer cancel()
	}
	problemApp := problem_app.NewProblemAppFromContext(l.svcCtx)
	statement, err := problemApp.GetLatestStatement(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return buildStatementResponse(ctx, statement), nil
}

func (l *GetStatementLogic) withTimeout() (context.Context, context.CancelFunc) {
	if l.svcCtx == nil || l.svcCtx.Config.Statement.Timeout <= 0 {
		return l.ctx, nil
	}
	ctx, cancel := context.WithTimeout(l.ctx, l.svcCtx.Config.Statement.Timeout)
	return ctx, cancel
}
