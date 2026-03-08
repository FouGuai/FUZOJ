// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"strings"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/svc"
	"fuzoj/services/contest_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProblemRemoveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProblemRemoveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProblemRemoveLogic {
	return &ProblemRemoveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProblemRemoveLogic) ProblemRemove(req *types.RemoveContestProblemRequest) (resp *types.SuccessResponse, err error) {
	if req == nil {
		return nil, appErr.ValidationError("request", "required")
	}
	if strings.TrimSpace(req.Id) == "" {
		return nil, appErr.ValidationError("contest_id", "required")
	}
	if req.ProblemId <= 0 {
		return nil, appErr.ValidationError("problem_id", "invalid")
	}
	if l.svcCtx.ContestStore == nil || l.svcCtx.ContestProblemStore == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("contest repository is not configured")
	}

	ctxTimeout := withTimeout(l.ctx, l.svcCtx.Config.Timeouts.DB)
	defer ctxTimeout.cancel()

	detail, err := loadContestOrError(ctxTimeout.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Logger.Errorf("load contest for remove problem failed contest_id=%s err=%v", req.Id, err)
		return nil, err
	}
	rule, _ := parseRuleJSON(detail.RuleJSON)
	if deriveContestStatus(detail.Status, time.Now(), detail.StartAt, detail.EndAt, rule.FreezeMinutesBeforeEnd) == "ended" {
		return nil, appErr.New(appErr.ContestEnded)
	}

	if err := l.svcCtx.ContestProblemStore.Remove(ctxTimeout.ctx, req.Id, req.ProblemId); err != nil {
		l.Logger.Errorf("remove contest problem failed contest_id=%s problem_id=%d err=%v", req.Id, req.ProblemId, err)
		return nil, appErr.Wrap(err, appErr.DatabaseError)
	}
	if l.svcCtx.ProblemRepo != nil {
		if err := l.svcCtx.ProblemRepo.InvalidateProblemCache(ctxTimeout.ctx, req.Id, req.ProblemId); err != nil {
			l.Logger.Errorf("invalidate contest problem cache failed contest_id=%s problem_id=%d err=%v", req.Id, req.ProblemId, err)
		}
	}
	return buildSuccessResponse(l.ctx, "Success"), nil

}
