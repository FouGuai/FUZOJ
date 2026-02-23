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

type CreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLogic {
	return &CreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateLogic) Create(req *types.CreateProblemRequest) (resp *types.CreateProblemResponse, err error) {
	problemApp := problem_app.NewProblemAppFromContext(l.svcCtx)
	id, err := problemApp.CreateProblem(l.ctx, problem_app.CreateInput{
		Title:   req.Title,
		OwnerID: req.OwnerId,
	})
	if err != nil {
		return nil, err
	}
	return buildCreateResponse(l.ctx, id), nil
}
