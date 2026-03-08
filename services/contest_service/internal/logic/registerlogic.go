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

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterContestRequest) (resp *types.SuccessResponse, err error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	if strings.TrimSpace(req.Id) == "" {
		return nil, appErr.ValidationError("contest_id", "required")
	}
	if req.UserId <= 0 {
		return nil, appErr.ValidationError("user_id", "invalid")
	}
	if l.svcCtx.ContestStore == nil || l.svcCtx.ContestParticipantStore == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("contest repository is not configured")
	}

	ctxTimeout := withTimeout(l.ctx, l.svcCtx.Config.Timeouts.DB)
	defer ctxTimeout.cancel()

	detail, err := loadContestOrError(ctxTimeout.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Logger.Errorf("load contest for register failed contest_id=%s err=%v", req.Id, err)
		return nil, err
	}
	if detail.Status == "draft" {
		return nil, appErr.New(appErr.RegistrationNotStarted)
	}
	if deriveContestStatus(detail.Status, time.Now(), detail.StartAt, detail.EndAt, 0) == "ended" {
		return nil, appErr.New(appErr.RegistrationClosed)
	}

	existing, err := l.svcCtx.ContestParticipantStore.Find(ctxTimeout.ctx, req.Id, req.UserId)
	if err != nil && !errors.Is(err, repository.ErrContestParticipantNotFound) {
		l.Logger.Errorf("find contest participant failed contest_id=%s user_id=%d err=%v", req.Id, req.UserId, err)
		return nil, appErr.Wrap(err, appErr.DatabaseError)
	}
	if err == nil && existing.Status == "registered" && existing.TeamID == req.TeamId {
		return buildSuccessResponse(l.ctx, "Success"), nil
	}

	now := time.Now().UTC()
	if err := l.svcCtx.ContestParticipantStore.Upsert(ctxTimeout.ctx, req.Id, repository.ContestParticipantItem{
		UserID:       req.UserId,
		TeamID:       req.TeamId,
		Status:       "registered",
		RegisteredAt: now,
	}); err != nil {
		l.Logger.Errorf("register contest failed contest_id=%s user_id=%d err=%v", req.Id, req.UserId, err)
		return nil, appErr.New(appErr.RegistrationFailed).WithMessage(err.Error())
	}
	if l.svcCtx.ParticipantRepo != nil {
		if err := l.svcCtx.ParticipantRepo.InvalidateParticipantCache(ctxTimeout.ctx, req.Id, req.UserId); err != nil {
			l.Logger.Errorf("invalidate participant cache failed contest_id=%s user_id=%d err=%v", req.Id, req.UserId, err)
		}
	}
	return buildSuccessResponse(l.ctx, "Success"), nil

}
