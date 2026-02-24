package logic

import (
	"context"

	contestpb "fuzoj/api/proto/contest"
	"fuzoj/services/contest_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContestProblemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContestProblemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContestProblemsLogic {
	return &GetContestProblemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetContestProblemsLogic) GetContestProblems(in *contestpb.ContestProblemsRequest) (*contestpb.ContestProblemsResponse, error) {
	// todo: add your logic here and delete this line

	return &contestpb.ContestProblemsResponse{}, nil
}
