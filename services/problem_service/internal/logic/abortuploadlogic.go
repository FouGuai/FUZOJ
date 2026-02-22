// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AbortUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAbortUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AbortUploadLogic {
	return &AbortUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AbortUploadLogic) AbortUpload(req *types.AbortUploadRequest) (resp *types.SuccessResponse, err error) {
	manager := newProblemManagerFromContext(l.svcCtx)
	if err := manager.AbortDataPackUpload(l.ctx, AbortUploadInput{
		ProblemID:       req.Id,
		UploadSessionID: req.UploadId,
	}); err != nil {
		return nil, err
	}
	return buildSuccessResponse(l.ctx, "Success"), nil
}
