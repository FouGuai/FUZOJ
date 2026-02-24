package logic

import (
	"context"

	contestpb "fuzoj/api/proto/contest"
	"fuzoj/services/contest_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContestMetaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContestMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContestMetaLogic {
	return &GetContestMetaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetContestMetaLogic) GetContestMeta(in *contestpb.ContestMetaRequest) (*contestpb.ContestMetaResponse, error) {
	// todo: add your logic here and delete this line

	return &contestpb.ContestMetaResponse{}, nil
}
