// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/submit_service/internal/logic/submit_app"
	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchStatusLogic {
	return &BatchStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchStatusLogic) BatchStatus(req *types.BatchStatusRequest) (resp *types.BatchStatusResponse, err error) {
	app, err := submit_app.NewSubmitApp(l.svcCtx)
	if err != nil {
		return nil, err
	}
	statuses, missing, err := app.GetStatusBatch(l.ctx, req.SubmissionIds)
	if err != nil {
		return nil, err
	}
	return buildBatchStatusResponse(l.ctx, statuses, missing), nil
}
