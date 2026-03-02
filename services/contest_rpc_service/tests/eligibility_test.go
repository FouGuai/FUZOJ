package tests

import (
	"context"
	"testing"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_rpc_service/internal/domain"
	"fuzoj/services/contest_rpc_service/internal/repository"
)

type stubContestRepo struct {
	meta repository.ContestMeta
	err  error
}

func (s stubContestRepo) GetMeta(ctx context.Context, contestID string) (repository.ContestMeta, error) {
	if s.err != nil {
		return repository.ContestMeta{}, s.err
	}
	if s.meta.ContestID == "" {
		return repository.ContestMeta{}, repository.ErrContestNotFound
	}
	return s.meta, nil
}

func (s stubContestRepo) InvalidateMetaCache(ctx context.Context, contestID string) error {
	return nil
}

type stubProblemRepo struct {
	exists bool
	err    error
}

func (s stubProblemRepo) HasProblem(ctx context.Context, contestID string, problemID int64) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if !s.exists {
		return false, repository.ErrContestProblemNotFound
	}
	return true, nil
}

func (s stubProblemRepo) InvalidateProblemCache(ctx context.Context, contestID string, problemID int64) error {
	return nil
}

type stubParticipantRepo struct {
	participant repository.ContestParticipant
	err         error
}

func (s stubParticipantRepo) GetParticipant(ctx context.Context, contestID string, userID int64) (repository.ContestParticipant, error) {
	if s.err != nil {
		return repository.ContestParticipant{}, s.err
	}
	if s.participant.ContestID == "" {
		return repository.ContestParticipant{}, repository.ErrParticipantNotFound
	}
	return s.participant, nil
}

func (s stubParticipantRepo) InvalidateParticipantCache(ctx context.Context, contestID string, userID int64) error {
	return nil
}

func TestEligibilityService_Check(t *testing.T) {
	now := time.Now()
	baseMeta := repository.ContestMeta{
		ContestID: "c1",
		StartAt:   now.Add(-time.Minute),
		EndAt:     now.Add(time.Minute),
	}

	tests := []struct {
		name      string
		meta      repository.ContestMeta
		metaErr   error
		problemOK bool
		part      repository.ContestParticipant
		expected  appErr.ErrorCode
		ok        bool
	}{
		{
			name:     "contest_not_found",
			meta:     repository.ContestMeta{},
			expected: appErr.ContestNotFound,
		},
		{
			name:     "not_started",
			meta:     repository.ContestMeta{ContestID: "c1", StartAt: now.Add(time.Minute), EndAt: now.Add(2 * time.Minute)},
			expected: appErr.ContestNotStarted,
		},
		{
			name:     "ended",
			meta:     repository.ContestMeta{ContestID: "c1", StartAt: now.Add(-2 * time.Minute), EndAt: now.Add(-time.Minute)},
			expected: appErr.ContestEnded,
		},
		{
			name:     "problem_not_found",
			meta:     baseMeta,
			expected: appErr.ProblemNotFound,
		},
		{
			name:      "not_registered",
			meta:      baseMeta,
			problemOK: true,
			expected:  appErr.NotRegistered,
		},
		{
			name:      "rejected",
			meta:      baseMeta,
			problemOK: true,
			part:      repository.ContestParticipant{ContestID: "c1", UserID: 1, Status: "rejected"},
			expected:  appErr.ContestAccessDenied,
		},
		{
			name:      "ok",
			meta:      baseMeta,
			problemOK: true,
			part:      repository.ContestParticipant{ContestID: "c1", UserID: 1, Status: "approved"},
			ok:        true,
			expected:  appErr.Success,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := domain.NewEligibilityService(
				stubContestRepo{meta: tt.meta, err: tt.metaErr},
				stubProblemRepo{exists: tt.problemOK},
				stubParticipantRepo{participant: tt.part},
			)
			result, err := svc.Check(context.Background(), domain.EligibilityRequest{
				ContestID: "c1",
				UserID:    1,
				ProblemID: 2,
				Now:       now,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.OK != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, result.OK)
			}
			if result.ErrorCode != tt.expected {
				t.Fatalf("expected code=%v, got %v", tt.expected, result.ErrorCode)
			}
		})
	}
}
