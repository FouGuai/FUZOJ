// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"
	"strings"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLogic {
	return &CreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateLogic) Create(req *types.CreateContestRequest) (resp *types.CreateContestResponse, err error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, appErr.ValidationError("title", "required")
	}
	startAt, err := parseTime(req.StartAt)
	if err != nil || startAt.IsZero() {
		return nil, appErr.ValidationError("start_at", "invalid")
	}
	endAt, err := parseTime(req.EndAt)
	if err != nil || endAt.IsZero() {
		return nil, appErr.ValidationError("end_at", "invalid")
	}
	if !startAt.Before(endAt) {
		return nil, appErr.ValidationError("time_range", "start_at_must_before_end_at")
	}
	if l.svcCtx.ContestStore == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("contest repository is not configured")
	}

	rule := normalizeRule(req.Rule)
	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		l.Logger.Errorf("marshal contest rule failed err=%v", err)
		return nil, appErr.Wrap(err, appErr.InvalidParams)
	}
	contestID := uuid.NewString()
	visibility := req.Visibility
	if strings.TrimSpace(visibility) == "" {
		visibility = "public"
	}

	ctxTimeout := withTimeout(l.ctx, l.svcCtx.Config.Timeouts.DB)
	defer ctxTimeout.cancel()

	l.Logger.Infof("create contest start contest_id=%s title=%s owner_id=%d", contestID, req.Title, req.OwnerId)
	if err := l.svcCtx.ContestStore.Create(ctxTimeout.ctx, repository.ContestCreateInput{
		ContestID:   contestID,
		Title:       req.Title,
		Description: req.Description,
		Status:      "draft",
		Visibility:  visibility,
		OwnerID:     req.OwnerId,
		OrgID:       req.OrgId,
		StartAt:     startAt,
		EndAt:       endAt,
		RuleJSON:    string(ruleJSON),
	}); err != nil {
		l.Logger.Errorf("create contest failed contest_id=%s err=%v", contestID, err)
		return nil, appErr.Wrap(err, appErr.ContestCreateFailed)
	}
	if l.svcCtx.ContestRepo != nil {
		if err := l.svcCtx.ContestRepo.InvalidateMetaCache(ctxTimeout.ctx, contestID); err != nil {
			l.Logger.Errorf("invalidate contest meta cache failed contest_id=%s err=%v", contestID, err)
		}
	}
	return buildCreateContestResponse(l.ctx, contestID), nil
}
