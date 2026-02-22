// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	appErr "fuzoj/pkg/errors"
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
	return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("submit service is not implemented")
}
