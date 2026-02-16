package service

import (
	"context"
	"fmt"

	"fuzoj/internal/problem/repository"
	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

// ProblemService handles problem meta queries.
type ProblemService struct {
	repo             repository.ProblemRepository
	cleanupPublisher *ProblemCleanupPublisher
}

// NewProblemService creates a new ProblemService.
func NewProblemService(repo repository.ProblemRepository, cleanupPublisher *ProblemCleanupPublisher) *ProblemService {
	return &ProblemService{repo: repo, cleanupPublisher: cleanupPublisher}
}

// CreateInput represents input for problem creation.
type CreateInput struct {
	Title   string
	OwnerID int64
}

// GetLatestMeta returns latest published meta for a problem.
func (s *ProblemService) GetLatestMeta(ctx context.Context, problemID int64) (repository.ProblemLatestMeta, error) {
	if problemID <= 0 {
		return repository.ProblemLatestMeta{}, pkgerrors.New(pkgerrors.InvalidParams)
	}

	meta, err := s.repo.GetLatestMeta(ctx, nil, problemID)
	if err != nil {
		if err == repository.ErrProblemNotFound {
			return repository.ProblemLatestMeta{}, pkgerrors.New(pkgerrors.ProblemNotFound)
		}
		return repository.ProblemLatestMeta{}, pkgerrors.Wrap(fmt.Errorf("get latest meta failed: %w", err), pkgerrors.DatabaseError)
	}
	return meta, nil
}

// CreateProblem creates a new problem with draft status.
func (s *ProblemService) CreateProblem(ctx context.Context, input CreateInput) (int64, error) {
	if input.Title == "" {
		return 0, pkgerrors.New(pkgerrors.InvalidParams)
	}

	problem := &repository.Problem{
		Title:   input.Title,
		OwnerID: input.OwnerID,
		Status:  repository.ProblemStatusDraft,
	}

	id, err := s.repo.Create(ctx, nil, problem)
	if err != nil {
		return 0, pkgerrors.Wrap(fmt.Errorf("create problem failed: %w", err), pkgerrors.ProblemCreateFailed)
	}
	return id, nil
}

// DeleteProblem deletes a problem by id.
func (s *ProblemService) DeleteProblem(ctx context.Context, problemID int64) error {
	if problemID <= 0 {
		return pkgerrors.New(pkgerrors.InvalidParams)
	}
	if err := s.repo.Delete(ctx, nil, problemID); err != nil {
		if err == repository.ErrProblemNotFound {
			return pkgerrors.New(pkgerrors.ProblemNotFound)
		}
		return pkgerrors.Wrap(fmt.Errorf("delete problem failed: %w", err), pkgerrors.ProblemDeleteFailed)
	}
	_ = s.repo.InvalidateLatestMetaCache(ctx, problemID)
	if s.cleanupPublisher != nil {
		if err := s.cleanupPublisher.PublishProblemDeleted(ctx, problemID); err != nil {
			logger.Warn(ctx, "publish cleanup event failed", zap.Int64("problem_id", problemID), zap.Error(err))
		}
	}
	return nil
}
