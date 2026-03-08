package eligibility

import (
	"context"
	"errors"
	"strings"
	"time"

	"fuzoj/pkg/contest/repository"
	appErr "fuzoj/pkg/errors"
)

type Request struct {
	ContestID string
	UserID    int64
	ProblemID int64
	Now       time.Time
}

type Result struct {
	OK        bool
	ErrorCode appErr.ErrorCode
	Message   string
}

type Service struct {
	contestRepo     repository.ContestRepository
	problemRepo     repository.ContestProblemRepository
	participantRepo repository.ContestParticipantRepository
}

func NewService(contestRepo repository.ContestRepository, problemRepo repository.ContestProblemRepository, participantRepo repository.ContestParticipantRepository) *Service {
	return &Service{
		contestRepo:     contestRepo,
		problemRepo:     problemRepo,
		participantRepo: participantRepo,
	}
}

func (s *Service) Check(ctx context.Context, req Request) (Result, error) {
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
		return Result{}, err
	}
	if now.Before(meta.StartAt) {
		return resultFromCode(appErr.ContestNotStarted), nil
	}
	if now.After(meta.EndAt) {
		return resultFromCode(appErr.ContestEnded), nil
	}
	if !canSubmitByContestStatus(meta.Status) {
		return resultFromCode(appErr.ContestAccessDenied), nil
	}

	if _, err := s.problemRepo.HasProblem(ctx, req.ContestID, req.ProblemID); err != nil {
		if errors.Is(err, repository.ErrContestProblemNotFound) {
			return resultFromCode(appErr.ProblemNotFound), nil
		}
		return Result{}, err
	}

	participant, err := s.participantRepo.GetParticipant(ctx, req.ContestID, req.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrParticipantNotFound) {
			return resultFromCode(appErr.NotRegistered), nil
		}
		return Result{}, err
	}
	if !isParticipantEligible(participant.Status) {
		return resultFromCode(appErr.ContestAccessDenied), nil
	}
	return Result{OK: true, ErrorCode: appErr.Success, Message: appErr.Success.Message()}, nil
}

func isParticipantEligible(status string) bool {
	switch status {
	case "registered", "approved":
		return true
	default:
		return false
	}
}

func canSubmitByContestStatus(status string) bool {
	switch status {
	case "published", "running", "frozen":
		return true
	default:
		return false
	}
}

func resultFromCode(code appErr.ErrorCode) Result {
	return Result{
		OK:        false,
		ErrorCode: code,
		Message:   code.Message(),
	}
}
