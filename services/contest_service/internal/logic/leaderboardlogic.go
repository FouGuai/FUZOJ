// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

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

func (l *LeaderboardLogic) Leaderboard(req *types.LeaderboardRequest) (resp *types.LeaderboardResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
