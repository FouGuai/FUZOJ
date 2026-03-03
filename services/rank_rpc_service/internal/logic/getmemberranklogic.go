package logic

import (
	"context"

	rankpb "fuzoj/api/proto/rank"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// GetMemberRankLogic handles rpc member rank queries.
type GetMemberRankLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMemberRankLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemberRankLogic {
	return &GetMemberRankLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMemberRankLogic) GetMemberRank(req *rankpb.GetMemberRankRequest) (*rankpb.MemberRankReply, error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	mode, err := NormalizeLeaderboardMode(req.Mode)
	if err != nil {
		return nil, err
	}
	return l.svcCtx.LeaderboardRepo.GetMember(l.ctx, req.ContestId, req.MemberId, mode)
}
