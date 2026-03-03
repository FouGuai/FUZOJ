package logic

import (
	"context"

	rankpb "fuzoj/api/proto/rank"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// GetLeaderboardLogic handles rpc leaderboard queries.
type GetLeaderboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLeaderboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLeaderboardLogic {
	return &GetLeaderboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetLeaderboardLogic) GetLeaderboard(req *rankpb.GetLeaderboardRequest) (*rankpb.LeaderboardReply, error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	mode, err := NormalizeLeaderboardMode(req.Mode)
	if err != nil {
		return nil, err
	}
	return l.svcCtx.LeaderboardRepo.GetPage(l.ctx, req.ContestId, int(req.Page), int(req.PageSize), mode)
}
