// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCompleteUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteUploadLogic {
	return &CompleteUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CompleteUploadLogic) CompleteUpload(req *types.CompleteUploadRequest) (resp *types.CompleteUploadResponse, err error) {
	manager := newProblemManagerFromContext(l.svcCtx)
	parts := make([]CompletedPartInput, 0, len(req.Parts))
	for _, part := range req.Parts {
		parts = append(parts, CompletedPartInput{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}
	output, err := manager.CompleteDataPackUpload(l.ctx, CompleteUploadInput{
		ProblemID:       req.Id,
		UploadSessionID: req.UploadId,
		Parts:           parts,
		ManifestJSON:    []byte(req.ManifestJson),
		ConfigJSON:      []byte(req.ConfigJson),
		ManifestHash:    req.ManifestHash,
		DataPackHash:    req.DataPackHash,
	})
	if err != nil {
		return nil, err
	}
	return buildCompleteUploadResponse(l.ctx, output), nil
}
