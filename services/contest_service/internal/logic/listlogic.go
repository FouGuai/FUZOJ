// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListLogic) List(req *types.ListContestsRequest) (resp *types.ListContestsResponse, err error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	if l.svcCtx.ContestStore == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("contest repository is not configured")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = l.svcCtx.Config.Contest.DefaultPageSize
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if l.svcCtx.Config.Contest.MaxPageSize > 0 && pageSize > l.svcCtx.Config.Contest.MaxPageSize {
		pageSize = l.svcCtx.Config.Contest.MaxPageSize
	}

	ctxTimeout := withTimeout(l.ctx, l.svcCtx.Config.Timeouts.DB)
	defer ctxTimeout.cancel()

	items, total, err := l.svcCtx.ContestStore.List(ctxTimeout.ctx, repository.ContestListFilter{
		Status:   req.Status,
		OwnerID:  req.OwnerId,
		OrgID:    req.OrgId,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		l.Logger.Errorf("list contests failed err=%v", err)
		return nil, appErr.Wrap(err, appErr.DatabaseError)
	}
	summaries := make([]types.ContestSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, types.ContestSummary{
			ContestId: item.ContestID,
			Title:     item.Title,
			Status:    item.Status,
			RuleType:  ruleTypeFromJSON(item.RuleJSON),
			StartAt:   formatTime(item.StartAt),
			EndAt:     formatTime(item.EndAt),
		})
	}
	payload := types.ListContestsPayload{
		Items: summaries,
		Page: types.PageInfo{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}
	return buildListContestsResponse(l.ctx, payload), nil
}
