package logic

import (
	"context"

	contestpb "fuzoj/api/proto/contest"
	"fuzoj/services/contest_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContestRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContestRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContestRuleLogic {
	return &GetContestRuleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetContestRuleLogic) GetContestRule(in *contestpb.ContestRuleRequest) (*contestpb.ContestRuleResponse, error) {
	// todo: add your logic here and delete this line

	return &contestpb.ContestRuleResponse{}, nil
}
