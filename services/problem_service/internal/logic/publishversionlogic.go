// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublishVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishVersionLogic {
	return &PublishVersionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishVersionLogic) PublishVersion(req *types.PublishVersionRequest) (resp *types.SuccessResponse, err error) {
	manager := newProblemManagerFromContext(l.svcCtx)
	if err := manager.PublishVersion(l.ctx, PublishInput{
		ProblemID: req.Id,
		Version:   req.Version,
	}); err != nil {
		return nil, err
	}
	return buildSuccessResponse(l.ctx, "Success"), nil
}
