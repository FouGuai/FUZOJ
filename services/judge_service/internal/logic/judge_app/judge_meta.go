package judge_app

import (
	"context"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/pmodel"

	"github.com/zeromicro/go-zero/core/logx"
)

func (s *JudgeApp) getProblemMeta(ctx context.Context, problemID int64) (pmodel.ProblemMeta, error) {
	if problemID <= 0 {
		return pmodel.ProblemMeta{}, appErr.ValidationError("problem_id", "required")
	}
	logger := logx.WithContext(ctx)
	now := time.Now()

	s.metaMu.Lock()
	entry, cached := s.metaCache[problemID]
	if cached && (s.metaTTL <= 0 || now.Before(entry.expiresAt)) {
		meta := entry.meta
		s.metaMu.Unlock()
		return meta, nil
	}
	var staleMeta pmodel.ProblemMeta
	hasStale := cached
	if hasStale {
		staleMeta = entry.meta
	}
	if call, ok := s.metaCalls[problemID]; ok {
		done := call.done
		s.metaMu.Unlock()
		select {
		case <-done:
			if call.err == nil {
				return call.meta, nil
			}
			if hasStale {
				logger.Errorf("problem meta rpc failed, using stale cache problem_id=%d err=%v", problemID, call.err)
				return staleMeta, nil
			}
			return pmodel.ProblemMeta{}, call.err
		case <-ctx.Done():
			return pmodel.ProblemMeta{}, ctx.Err()
		}
	}
	call := &metaCall{done: make(chan struct{})}
	s.metaCalls[problemID] = call
	s.metaMu.Unlock()

	defer func() {
		s.metaMu.Lock()
		close(call.done)
		delete(s.metaCalls, problemID)
		s.metaMu.Unlock()
	}()

	ctxRPC := ctx
	if s.problemTimeout > 0 {
		var cancel context.CancelFunc
		ctxRPC, cancel = context.WithTimeout(ctx, s.problemTimeout)
		defer cancel()
	}
	meta, err := s.problemClient.GetLatest(ctxRPC, problemID)
	if err != nil {
		call.err = err
		if hasStale {
			logger.Errorf("problem meta rpc failed, using stale cache problem_id=%d err=%v", problemID, err)
			call.meta = staleMeta
			call.err = nil
			return staleMeta, nil
		}
		return pmodel.ProblemMeta{}, err
	}
	call.meta = meta
	s.metaMu.Lock()
	if s.metaTTL > 0 {
		s.metaCache[problemID] = metaEntry{meta: meta, expiresAt: time.Now().Add(s.metaTTL)}
	} else {
		s.metaCache[problemID] = metaEntry{meta: meta, expiresAt: time.Time{}}
	}
	s.metaMu.Unlock()
	return meta, nil
}
