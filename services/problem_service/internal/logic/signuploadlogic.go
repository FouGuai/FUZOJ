// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SignUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSignUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignUploadLogic {
	return &SignUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SignUploadLogic) SignUpload(req *types.SignPartsRequest) (resp *types.SignPartsResponse, err error) {
	manager := newProblemManagerFromContext(l.svcCtx)
	output, err := manager.SignUploadParts(l.ctx, SignPartsInput{
		ProblemID:       req.Id,
		UploadSessionID: req.UploadId,
		PartNumbers:     req.PartNumbers,
	})
	if err != nil {
		return nil, err
	}
	return buildSignPartsResponse(l.ctx, output), nil
}
