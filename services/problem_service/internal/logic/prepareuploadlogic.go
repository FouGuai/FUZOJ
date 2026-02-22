// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PrepareUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPrepareUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrepareUploadLogic {
	return &PrepareUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PrepareUploadLogic) PrepareUpload(req *types.PrepareUploadRequest) (resp *types.PrepareUploadResponse, err error) {
	manager := newProblemManagerFromContext(l.svcCtx)
	output, err := manager.PrepareDataPackUpload(l.ctx, PrepareUploadInput{
		ProblemID:         req.Id,
		IdempotencyKey:    req.IdempotencyKey,
		ExpectedSizeBytes: req.ExpectedSizeBytes,
		ExpectedSHA256:    req.ExpectedSha256,
		ContentType:       req.ContentType,
		CreatedBy:         req.CreatedBy,
		ClientType:        req.ClientType,
		UploadStrategy:    req.UploadStrategy,
	})
	if err != nil {
		return nil, err
	}
	return buildPrepareUploadResponse(l.ctx, output), nil
}
