package server

import (
	"context"

	rankpb "fuzoj/api/proto/rank"
	"fuzoj/services/rank_rpc_service/internal/logic"
	"fuzoj/services/rank_rpc_service/internal/svc"
)

// RankRpcServer implements rank rpc service.
type RankRpcServer struct {
	svcCtx *svc.ServiceContext
	rankpb.UnimplementedRankRpcServer
}

func NewRankRpcServer(svcCtx *svc.ServiceContext) *RankRpcServer {
	return &RankRpcServer{svcCtx: svcCtx}
}

func (s *RankRpcServer) GetLeaderboard(ctx context.Context, req *rankpb.GetLeaderboardRequest) (*rankpb.LeaderboardReply, error) {
	l := logic.NewGetLeaderboardLogic(ctx, s.svcCtx)
	return l.GetLeaderboard(req)
}

func (s *RankRpcServer) GetMemberRank(ctx context.Context, req *rankpb.GetMemberRankRequest) (*rankpb.MemberRankReply, error) {
	l := logic.NewGetMemberRankLogic(ctx, s.svcCtx)
	return l.GetMemberRank(req)
}
