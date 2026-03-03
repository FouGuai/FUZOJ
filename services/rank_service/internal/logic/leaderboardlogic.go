package logic

import (
	"context"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/rank_service/internal/svc"
	"fuzoj/services/rank_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// LeaderboardLogic handles leaderboard queries.
type LeaderboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLeaderboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LeaderboardLogic {
	return &LeaderboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LeaderboardLogic) Leaderboard(req *types.LeaderboardRequest) (*types.LeaderboardResponse, error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	if req.Id == "" {
		return nil, appErr.ValidationError("contest_id", "required")
	}
	mode, err := NormalizeLeaderboardMode(req.Mode)
	if err != nil {
		return nil, err
	}
	payload, err := l.svcCtx.LeaderboardRepo.GetPage(l.ctx, req.Id, req.Page, req.PageSize, mode)
	if err != nil {
		return nil, err
	}
	return &types.LeaderboardResponse{
		Code:    0,
		Message: "ok",
		Data:    payload,
	}, nil
}
