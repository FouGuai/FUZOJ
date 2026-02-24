// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LeaderboardReplayLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLeaderboardReplayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LeaderboardReplayLogic {
	return &LeaderboardReplayLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LeaderboardReplayLogic) LeaderboardReplay(req *types.LeaderboardReplayRequest) (resp *types.LeaderboardResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
