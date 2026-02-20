package service

import (
	"context"
	"time"

	"fuzoj/internal/judge/model"
	appErr "fuzoj/pkg/errors"
)

func (s *Service) getProblemMeta(ctx context.Context, problemID int64) (model.ProblemMeta, error) {
	if problemID <= 0 {
		return model.ProblemMeta{}, appErr.ValidationError("problem_id", "required")
	}
	now := time.Now()
	if s.metaTTL > 0 {
		s.metaMu.Lock()
		entry, ok := s.metaCache[problemID]
		if ok && now.Before(entry.expiresAt) {
			meta := entry.meta
			s.metaMu.Unlock()
			return meta, nil
		}
		s.metaMu.Unlock()
	}

	ctxRPC := ctx
	if s.problemTimeout > 0 {
		var cancel context.CancelFunc
		ctxRPC, cancel = context.WithTimeout(ctx, s.problemTimeout)
		defer cancel()
	}
	meta, err := s.problemClient.GetLatest(ctxRPC, problemID)
	if err != nil {
		return model.ProblemMeta{}, err
	}
	if s.metaTTL > 0 {
		s.metaMu.Lock()
		s.metaCache[problemID] = metaEntry{meta: meta, expiresAt: now.Add(s.metaTTL)}
		s.metaMu.Unlock()
	}
	return meta, nil
}
