package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_rpc_service/internal/repository"
)

type EligibilityRequest struct {
	ContestID string
	UserID    int64
	ProblemID int64
	Now       time.Time
}

type EligibilityResult struct {
	OK        bool
	ErrorCode appErr.ErrorCode
	Message   string
}

type EligibilityService struct {
	contestRepo     repository.ContestRepository
	problemRepo     repository.ContestProblemRepository
	participantRepo repository.ContestParticipantRepository
}

func NewEligibilityService(contestRepo repository.ContestRepository, problemRepo repository.ContestProblemRepository, participantRepo repository.ContestParticipantRepository) *EligibilityService {
	return &EligibilityService{
		contestRepo:     contestRepo,
		problemRepo:     problemRepo,
		participantRepo: participantRepo,
	}
}

func (s *EligibilityService) Check(ctx context.Context, req EligibilityRequest) (EligibilityResult, error) {
	if strings.TrimSpace(req.ContestID) == "" {
		return resultFromCode(appErr.InvalidParams), nil
	}
	if req.UserID <= 0 || req.ProblemID <= 0 {
		return resultFromCode(appErr.InvalidParams), nil
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	meta, err := s.contestRepo.GetMeta(ctx, req.ContestID)
	if err != nil {
		if errors.Is(err, repository.ErrContestNotFound) {
			return resultFromCode(appErr.ContestNotFound), nil
		}
		return EligibilityResult{}, err
	}
	if now.Before(meta.StartAt) {
		return resultFromCode(appErr.ContestNotStarted), nil
	}
	if now.After(meta.EndAt) {
		return resultFromCode(appErr.ContestEnded), nil
	}

	if _, err := s.problemRepo.HasProblem(ctx, req.ContestID, req.ProblemID); err != nil {
		if errors.Is(err, repository.ErrContestProblemNotFound) {
			return resultFromCode(appErr.ProblemNotFound), nil
		}
		return EligibilityResult{}, err
	}

	participant, err := s.participantRepo.GetParticipant(ctx, req.ContestID, req.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrParticipantNotFound) {
			return resultFromCode(appErr.NotRegistered), nil
		}
		return EligibilityResult{}, err
	}
	if !isParticipantEligible(participant.Status) {
		return resultFromCode(appErr.ContestAccessDenied), nil
	}
	return EligibilityResult{OK: true, ErrorCode: appErr.Success, Message: appErr.Success.Message()}, nil
}

func isParticipantEligible(status string) bool {
	switch status {
	case "registered", "approved":
		return true
	default:
		return false
	}
}

func resultFromCode(code appErr.ErrorCode) EligibilityResult {
	return EligibilityResult{
		OK:        false,
		ErrorCode: code,
		Message:   code.Message(),
	}
}
