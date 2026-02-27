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
	l.Logger.Infof("complete upload request received problem_id=%d upload_id=%d parts=%d", req.Id, req.UploadId, len(req.Parts))
	problemApp := problem_app.NewProblemAppFromContext(l.svcCtx)
	parts := make([]problem_app.CompletedPartInput, 0, len(req.Parts))
	for _, part := range req.Parts {
		parts = append(parts, problem_app.CompletedPartInput{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}
	output, err := problemApp.CompleteDataPackUpload(l.ctx, problem_app.CompleteUploadInput{
		ProblemID:       req.Id,
		UploadSessionID: req.UploadId,
		Parts:           parts,
		ManifestJSON:    []byte(req.ManifestJson),
		ConfigJSON:      []byte(req.ConfigJson),
		ManifestHash:    req.ManifestHash,
		DataPackHash:    req.DataPackHash,
	})
	if err != nil {
		l.Logger.Errorf("complete upload failed problem_id=%d upload_id=%d err=%v", req.Id, req.UploadId, err)
		return nil, err
	}
	l.Logger.Infof("complete upload succeeded problem_id=%d upload_id=%d version=%d", req.Id, req.UploadId, output.Version)
	return buildCompleteUploadResponse(l.ctx, output), nil
}
