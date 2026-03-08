// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogic {
	return &GetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetLogic) Get(req *types.GetContestRequest) (resp *types.GetContestResponse, err error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	if strings.TrimSpace(req.Id) == "" {
		return nil, appErr.ValidationError("contest_id", "required")
	}
	if l.svcCtx.ContestStore == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("contest repository is not configured")
	}

	ctxTimeout := withTimeout(l.ctx, l.svcCtx.Config.Timeouts.DB)
	defer ctxTimeout.cancel()

	detail, err := l.svcCtx.ContestStore.Get(ctxTimeout.ctx, req.Id)
	if err != nil {
		if errors.Is(err, repository.ErrContestNotFound) {
			return nil, appErr.New(appErr.ContestNotFound)
		}
		l.Logger.Errorf("get contest failed contest_id=%s err=%v", req.Id, err)
		return nil, appErr.Wrap(err, appErr.DatabaseError)
	}
	rule, err := parseRuleJSON(detail.RuleJSON)
	if err != nil {
		l.Logger.Errorf("parse contest rule failed contest_id=%s err=%v", req.Id, err)
	}
	status := deriveContestStatus(detail.Status, time.Now(), detail.StartAt, detail.EndAt, rule.FreezeMinutesBeforeEnd)
	respDetail := types.ContestDetail{
		ContestId:   detail.ContestID,
		Title:       detail.Title,
		Description: detail.Description,
		Status:      status,
		Visibility:  detail.Visibility,
		OwnerId:     detail.OwnerID,
		OrgId:       detail.OrgID,
		StartAt:     formatTime(detail.StartAt),
		EndAt:       formatTime(detail.EndAt),
		Rule:        rule,
		CreatedAt:   formatTime(detail.CreatedAt),
		UpdatedAt:   formatTime(detail.UpdatedAt),
	}
	return buildGetContestResponse(l.ctx, respDetail), nil
}
