package logic

import (
	"context"
	"time"

	contestpb "fuzoj/api/proto/contest"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckParticipationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckParticipationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckParticipationLogic {
	return &CheckParticipationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckParticipationLogic) CheckParticipation(in *contestpb.CheckParticipationRequest) (*contestpb.CheckParticipationResponse, error) {
	logx.WithContext(l.ctx).Infof("check participation start contest_id=%s user_id=%d problem_id=%d", in.GetContestId(), in.GetUserId(), in.GetProblemId())
	if l.svcCtx.EligibilityService == nil {
		logx.WithContext(l.ctx).Error("eligibility service is not configured")
		return &contestpb.CheckParticipationResponse{
			Ok:           false,
			ErrorCode:    int32(appErr.ServiceUnavailable),
			ErrorMessage: appErr.ServiceUnavailable.Message(),
		}, nil
	}
	result, err := l.svcCtx.EligibilityService.Check(l.ctx, svc.EligibilityRequestFromProto(in, time.Now()))
	if err != nil {
		logx.WithContext(l.ctx).Errorf("check participation failed: %v", err)
		return nil, err
	}
	return &contestpb.CheckParticipationResponse{
		Ok:           result.OK,
		ErrorCode:    int32(result.ErrorCode),
		ErrorMessage: result.Message,
	}, nil
}
