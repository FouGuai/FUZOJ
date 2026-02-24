package logic

import (
	"context"

	contestpb "fuzoj/api/proto/contest"
	"fuzoj/services/contest_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckParticipationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckParticipationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckParticipationLogic {
	return &CheckParticipationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckParticipationLogic) CheckParticipation(in *contestpb.CheckParticipationRequest) (*contestpb.CheckParticipationResponse, error) {
	// todo: add your logic here and delete this line

	return &contestpb.CheckParticipationResponse{}, nil
}
