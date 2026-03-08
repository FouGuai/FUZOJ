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

type ParticipantsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewParticipantsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ParticipantsLogic {
	return &ParticipantsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ParticipantsLogic) Participants(req *types.ListParticipantsRequest) (resp *types.ListParticipantsResponse, err error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	if strings.TrimSpace(req.Id) == "" {
		return nil, appErr.ValidationError("contest_id", "required")
	}
	if l.svcCtx.ContestStore == nil || l.svcCtx.ContestParticipantStore == nil {
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

	if _, err := loadContestOrError(ctxTimeout.ctx, l.svcCtx, req.Id); err != nil {
		l.Logger.Errorf("load contest for list participants failed contest_id=%s err=%v", req.Id, err)
		return nil, err
	}

	items, total, err := l.svcCtx.ContestParticipantStore.List(ctxTimeout.ctx, req.Id, page, pageSize)
	if err != nil {
		l.Logger.Errorf("list participants failed contest_id=%s err=%v", req.Id, err)
		return nil, appErr.Wrap(err, appErr.DatabaseError)
	}
	respItems := make([]types.ParticipantInfo, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, types.ParticipantInfo{
			UserId:   item.UserID,
			TeamId:   item.TeamID,
			Status:   item.Status,
			JoinedAt: formatTime(item.RegisteredAt),
		})
	}
	return &types.ListParticipantsResponse{
		Code:    int(appErr.Success),
		Message: appErr.Success.Message(),
		Data: types.ListParticipantsPayload{
			Items: respItems,
			Page: types.PageInfo{
				Page:     page,
				PageSize: pageSize,
				Total:    total,
			},
		},
		TraceId: traceIDFromContext(l.ctx),
	}, nil

}
