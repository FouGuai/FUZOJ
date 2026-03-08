// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"strings"

	appErr "fuzoj/pkg/errors"
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
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	if strings.TrimSpace(req.Id) == "" {
		return nil, appErr.ValidationError("contest_id", "required")
	}
	if l.svcCtx.ContestStore == nil || l.svcCtx.ContestProblemStore == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("contest repository is not configured")
	}

	ctxTimeout := withTimeout(l.ctx, l.svcCtx.Config.Timeouts.DB)
	defer ctxTimeout.cancel()

	if _, err := loadContestOrError(ctxTimeout.ctx, l.svcCtx, req.Id); err != nil {
		l.Logger.Errorf("load contest for list problems failed contest_id=%s err=%v", req.Id, err)
		return nil, err
	}
	items, err := l.svcCtx.ContestProblemStore.List(ctxTimeout.ctx, req.Id)
	if err != nil {
		l.Logger.Errorf("list contest problems failed contest_id=%s err=%v", req.Id, err)
		return nil, appErr.Wrap(err, appErr.DatabaseError)
	}
	respItems := make([]types.ContestProblemInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, types.ContestProblemInfo{
			ProblemId: item.ProblemID,
			Order:     item.Order,
			Score:     item.Score,
			Visible:   item.Visible,
			Version:   item.Version,
		})
	}
	return &types.ListContestProblemsResponse{
		Code:    int(appErr.Success),
		Message: appErr.Success.Message(),
		Data:    respItems,
		TraceId: traceIDFromContext(l.ctx),
	}, nil

}
