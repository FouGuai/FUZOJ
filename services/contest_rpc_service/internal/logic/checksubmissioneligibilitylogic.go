package logic

import (
	"context"
	"time"

	contestpb "fuzoj/api/proto/contest"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckSubmissionEligibilityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckSubmissionEligibilityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckSubmissionEligibilityLogic {
	return &CheckSubmissionEligibilityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckSubmissionEligibilityLogic) CheckSubmissionEligibility(in *contestpb.CheckSubmissionEligibilityRequest) (*contestpb.CheckSubmissionEligibilityResponse, error) {
	logx.WithContext(l.ctx).Infof("check submission eligibility start contest_id=%s user_id=%d problem_id=%d", in.GetContestId(), in.GetUserId(), in.GetProblemId())
	if l.svcCtx.EligibilityService == nil {
		logx.WithContext(l.ctx).Error("eligibility service is not configured")
		return &contestpb.CheckSubmissionEligibilityResponse{
			Ok:           false,
			ErrorCode:    int32(appErr.ServiceUnavailable),
			ErrorMessage: appErr.ServiceUnavailable.Message(),
		}, nil
	}
	now := time.Unix(in.GetSubmitAt(), 0)
	if in.GetSubmitAt() == 0 {
		now = time.Now()
	}
	result, err := l.svcCtx.EligibilityService.Check(l.ctx, svc.EligibilityRequestFromProto(in, now))
	if err != nil {
		logx.WithContext(l.ctx).Errorf("check submission eligibility failed: %v", err)
		return nil, err
	}
	return &contestpb.CheckSubmissionEligibilityResponse{
		Ok:           result.OK,
		ErrorCode:    int32(result.ErrorCode),
		ErrorMessage: result.Message,
	}, nil
}
