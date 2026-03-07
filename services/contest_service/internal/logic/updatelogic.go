// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogic {
	return &UpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateLogic) Update(req *types.UpdateContestRequest) (resp *types.SuccessResponse, err error) {
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

	existing, err := l.svcCtx.ContestStore.Get(ctxTimeout.ctx, req.Id)
	if err != nil {
		if errors.Is(err, repository.ErrContestNotFound) {
			return nil, appErr.New(appErr.ContestNotFound)
		}
		l.Logger.Errorf("load contest failed contest_id=%s err=%v", req.Id, err)
		return nil, appErr.Wrap(err, appErr.DatabaseError)
	}

	update := repository.ContestUpdate{}
	if strings.TrimSpace(req.Title) != "" {
		title := req.Title
		update.Title = &title
	}
	if req.Description != "" {
		desc := req.Description
		update.Description = &desc
	}
	if strings.TrimSpace(req.Visibility) != "" {
		vis := req.Visibility
		update.Visibility = &vis
	}

	var startAt *time.Time
	var endAt *time.Time
	if strings.TrimSpace(req.StartAt) != "" {
		parsed, err := parseTime(req.StartAt)
		if err != nil || parsed.IsZero() {
			return nil, appErr.ValidationError("start_at", "invalid")
		}
		startAt = &parsed
		update.StartAt = startAt
	}
	if strings.TrimSpace(req.EndAt) != "" {
		parsed, err := parseTime(req.EndAt)
		if err != nil || parsed.IsZero() {
			return nil, appErr.ValidationError("end_at", "invalid")
		}
		endAt = &parsed
		update.EndAt = endAt
	}

	if startAt != nil || endAt != nil {
		newStart := existing.StartAt
		newEnd := existing.EndAt
		if startAt != nil {
			newStart = *startAt
		}
		if endAt != nil {
			newEnd = *endAt
		}
		if !newStart.Before(newEnd) {
			return nil, appErr.ValidationError("time_range", "start_at_must_before_end_at")
		}
	}

	if hasRuleUpdate(req.Rule) {
		existingRule, err := parseRuleJSON(existing.RuleJSON)
		if err != nil {
			l.Logger.Errorf("parse existing rule failed contest_id=%s err=%v", req.Id, err)
		}
		rule, merged := mergeRuleUpdate(existingRule, req.Rule)
		if !merged {
			return buildSuccessResponse(l.ctx, "Success"), nil
		}
		rule = normalizeRule(rule)
		ruleJSON, err := json.Marshal(rule)
		if err != nil {
			l.Logger.Errorf("marshal contest rule failed contest_id=%s err=%v", req.Id, err)
			return nil, appErr.Wrap(err, appErr.InvalidParams)
		}
		ruleJSONStr := string(ruleJSON)
		update.RuleJSON = &ruleJSONStr
	}

	if update.Title == nil && update.Description == nil && update.Visibility == nil && update.StartAt == nil && update.EndAt == nil && update.RuleJSON == nil {
		return buildSuccessResponse(l.ctx, "Success"), nil
	}

	if err := l.svcCtx.ContestStore.Update(ctxTimeout.ctx, req.Id, update); err != nil {
		if errors.Is(err, repository.ErrContestNotFound) {
			return nil, appErr.New(appErr.ContestNotFound)
		}
		l.Logger.Errorf("update contest failed contest_id=%s err=%v", req.Id, err)
		return nil, appErr.Wrap(err, appErr.ContestUpdateFailed)
	}
	if l.svcCtx.ContestRepo != nil {
		if err := l.svcCtx.ContestRepo.InvalidateMetaCache(ctxTimeout.ctx, req.Id); err != nil {
			l.Logger.Errorf("invalidate contest meta cache failed contest_id=%s err=%v", req.Id, err)
		}
	}
	if l.svcCtx.ContestStore != nil {
		if err := l.svcCtx.ContestStore.InvalidateDetailCache(ctxTimeout.ctx, req.Id); err != nil {
			l.Logger.Errorf("invalidate contest detail cache failed contest_id=%s err=%v", req.Id, err)
		}
	}
	return buildSuccessResponse(l.ctx, "Success"), nil
}
