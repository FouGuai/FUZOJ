// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"
	"strings"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishLogic {
	return &PublishLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishLogic) Publish(req *types.GetContestRequest) (resp *types.SuccessResponse, err error) {
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
		l.Logger.Errorf("get contest for publish failed contest_id=%s err=%v", req.Id, err)
		return nil, appErr.Wrap(err, appErr.DatabaseError)
	}
	if detail.Status == "draft" {
		next := "published"
		if err := l.svcCtx.ContestStore.Update(ctxTimeout.ctx, req.Id, repository.ContestUpdate{Status: &next}); err != nil {
			l.Logger.Errorf("publish contest failed contest_id=%s err=%v", req.Id, err)
			return nil, appErr.Wrap(err, appErr.ContestUpdateFailed)
		}
		if l.svcCtx.ContestRepo != nil {
			if err := l.svcCtx.ContestRepo.InvalidateMetaCache(ctxTimeout.ctx, req.Id); err != nil {
				l.Logger.Errorf("invalidate contest meta cache failed contest_id=%s err=%v", req.Id, err)
			}
		}
		if err := l.svcCtx.ContestStore.InvalidateDetailCache(ctxTimeout.ctx, req.Id); err != nil {
			l.Logger.Errorf("invalidate contest detail cache failed contest_id=%s err=%v", req.Id, err)
		}
		return buildSuccessResponse(l.ctx, "Success"), nil
	}
	if detail.Status == "published" || detail.Status == "running" || detail.Status == "frozen" {
		return buildSuccessResponse(l.ctx, "Success"), nil
	}
	return nil, appErr.New(appErr.ContestAccessDenied).WithMessage("contest status does not allow publish")

}
