// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProblemListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProblemListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProblemListLogic {
	return &ProblemListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProblemListLogic) ProblemList(req *types.ListContestProblemsRequest) (resp *types.ListContestProblemsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
