package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	"fuzoj/internal/judge/cache"
	"fuzoj/internal/judge/model"
	"fuzoj/internal/judge/problemclient"
	"fuzoj/internal/judge/repository"
	"fuzoj/internal/judge/sandbox"
	"fuzoj/internal/judge/sandbox/result"
	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"

	"go.uber.org/zap"
)

// Service handles judge tasks.
type Service struct {
	worker         *sandbox.Worker
	statusRepo     *repository.StatusRepository
	problemClient  *problemclient.Client
	dataCache      *cache.DataPackCache
	storage        storage.ObjectStorage
	sourceBucket   string
	workRoot       string
	workerTimeout  time.Duration
	problemTimeout time.Duration
	storageTimeout time.Duration
	statusTimeout  time.Duration
	metaTTL        time.Duration
	sem            chan struct{}

	metaMu    sync.Mutex
	metaCache map[int64]metaEntry
}

type metaEntry struct {
	meta      model.ProblemMeta
	expiresAt time.Time
}

// Config holds service dependencies and settings.
type Config struct {
	Worker         *sandbox.Worker
	StatusRepo     *repository.StatusRepository
	ProblemClient  *problemclient.Client
	DataCache      *cache.DataPackCache
	Storage        storage.ObjectStorage
	SourceBucket   string
	WorkRoot       string
	WorkerTimeout  time.Duration
	ProblemTimeout time.Duration
	StorageTimeout time.Duration
	StatusTimeout  time.Duration
	MetaTTL        time.Duration
	WorkerPoolSize int
}

// NewService creates a new judge service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Worker == nil {
		return nil, fmt.Errorf("worker is required")
	}
	if cfg.StatusRepo == nil {
		return nil, fmt.Errorf("status repository is required")
	}
	if cfg.ProblemClient == nil {
		return nil, fmt.Errorf("problem client is required")
	}
	if cfg.DataCache == nil {
		return nil, fmt.Errorf("data cache is required")
	}
	if cfg.Storage == nil {
		return nil, fmt.Errorf("storage client is required")
	}
	if cfg.WorkRoot == "" {
		return nil, fmt.Errorf("work root is required")
	}
	poolSize := cfg.WorkerPoolSize
	if poolSize <= 0 {
		poolSize = 1
	}
	return &Service{
		worker:         cfg.Worker,
		statusRepo:     cfg.StatusRepo,
		problemClient:  cfg.ProblemClient,
		dataCache:      cfg.DataCache,
		storage:        cfg.Storage,
		sourceBucket:   cfg.SourceBucket,
		workRoot:       cfg.WorkRoot,
		workerTimeout:  cfg.WorkerTimeout,
		problemTimeout: cfg.ProblemTimeout,
		storageTimeout: cfg.StorageTimeout,
		statusTimeout:  cfg.StatusTimeout,
		metaTTL:        cfg.MetaTTL,
		sem:            make(chan struct{}, poolSize),
		metaCache:      make(map[int64]metaEntry),
	}, nil
}

// HandleMessage processes a judge task message.
func (s *Service) HandleMessage(ctx context.Context, msg *mq.Message) error {
	if msg == nil {
		return appErr.New(appErr.InvalidParams).WithMessage("message is nil")
	}
	var payload model.JudgeMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		return appErr.Wrapf(err, appErr.InvalidParams, "decode message failed")
	}
	if payload.SubmissionID == "" || payload.ProblemID <= 0 || payload.LanguageID == "" || payload.SourceKey == "" {
		return appErr.New(appErr.InvalidParams).WithMessage("message missing required fields")
	}

	now := time.Now().Unix()
	pending := model.JudgeStatusResponse{
		SubmissionID: payload.SubmissionID,
		Status:       result.StatusPending,
		Timestamps:   result.Timestamps{ReceivedAt: now},
		Progress:     model.Progress{TotalTests: 0, DoneTests: 0},
	}
	if err := s.saveStatus(ctx, pending); err != nil {
		return err
	}

	if err := s.acquireSlot(ctx, payload.SubmissionID); err != nil {
		return err
	}
	defer s.releaseSlot()

	running := pending
	running.Status = result.StatusRunning
	running.Timestamps.ReceivedAt = pending.Timestamps.ReceivedAt
	running.Timestamps.FinishedAt = 0
	if err := s.saveStatus(ctx, running); err != nil {
		return err
	}

	meta, err := s.getProblemMeta(ctx, payload.ProblemID)
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, err)
	}
	dataPath, err := s.dataCache.Get(ctx, meta)
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, err)
	}

	manifest, err := model.LoadManifest(filepath.Join(dataPath, "manifest.json"))
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, appErr.Wrapf(err, appErr.JudgeSystemError, "load manifest failed"))
	}
	config, err := model.LoadProblemConfig(filepath.Join(dataPath, "config.json"))
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, appErr.Wrapf(err, appErr.JudgeSystemError, "load config failed"))
	}

	compileFlags, defaultLimits := resolveLanguageConfig(config, payload.LanguageID)
	compileFlags = append(compileFlags, payload.ExtraCompileFlags...)

	sourcePath, err := s.downloadSource(ctx, payload)
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, err)
	}

	tests, subtasks, err := buildTestcases(manifest, dataPath, defaultLimits)
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, err)
	}

	running.Progress.TotalTests = len(tests)
	if err := s.saveStatus(ctx, running); err != nil {
		return err
	}

	judgeReq := sandbox.JudgeRequest{
		SubmissionID:      payload.SubmissionID,
		LanguageID:        payload.LanguageID,
		WorkRoot:          s.workRoot,
		SourcePath:        sourcePath,
		Tests:             tests,
		Subtasks:          subtasks,
		ExtraCompileFlags: compileFlags,
		ProblemID:         strconv.FormatInt(payload.ProblemID, 10),
		ContestID:         payload.ContestID,
		UserID:            payload.UserID,
		Priority:          payload.Priority,
	}

	ctxWorker := ctx
	if s.workerTimeout > 0 {
		var cancel context.CancelFunc
		ctxWorker, cancel = context.WithTimeout(ctx, s.workerTimeout)
		defer cancel()
	}

	res, err := s.worker.Execute(ctxWorker, judgeReq)
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, err)
	}

	finished := model.JudgeStatusResponse{
		SubmissionID: payload.SubmissionID,
		Status:       res.Status,
		Verdict:      res.Verdict,
		Score:        res.Summary.TotalScore,
		Language:     res.Language,
		Compile:      res.Compile,
		Tests:        res.Tests,
		Summary:      res.Summary,
		Timestamps: result.Timestamps{
			ReceivedAt: running.Timestamps.ReceivedAt,
			FinishedAt: time.Now().Unix(),
		},
		Progress: model.Progress{TotalTests: len(res.Tests), DoneTests: len(res.Tests)},
	}
	if err := s.saveStatus(ctx, finished); err != nil {
		return err
	}
	return nil
}

func (s *Service) acquireSlot(ctx context.Context, submissionID string) error {
	select {
	case s.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return appErr.New(appErr.JudgeQueueFull).WithMessage("worker pool is full")
	}
}

func (s *Service) releaseSlot() {
	select {
	case <-s.sem:
	default:
	}
}

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

func (s *Service) downloadSource(ctx context.Context, payload model.JudgeMessage) (string, error) {
	submissionDir := filepath.Join(s.workRoot, payload.SubmissionID, "source")
	if err := os.MkdirAll(submissionDir, 0755); err != nil {
		return "", appErr.Wrapf(err, appErr.JudgeSystemError, "create source dir failed")
	}
	filePath := filepath.Join(submissionDir, "source.code")
	ctxStorage := ctx
	if s.storageTimeout > 0 {
		var cancel context.CancelFunc
		ctxStorage, cancel = context.WithTimeout(ctx, s.storageTimeout)
		defer cancel()
	}
	reader, err := s.storage.GetObject(ctxStorage, s.sourceBucket, payload.SourceKey)
	if err != nil {
		return "", appErr.Wrapf(err, appErr.JudgeSystemError, "download source failed")
	}
	defer reader.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return "", appErr.Wrapf(err, appErr.JudgeSystemError, "create source file failed")
	}
	defer file.Close()

	hasher := sha256.New()
	tee := io.TeeReader(reader, hasher)
	if _, err := io.Copy(file, tee); err != nil {
		return "", appErr.Wrapf(err, appErr.JudgeSystemError, "write source file failed")
	}
	if payload.SourceHash != "" {
		actual := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actual, payload.SourceHash) {
			return "", appErr.New(appErr.InvalidParams).WithMessage("source hash mismatch")
		}
	}
	return filePath, nil
}

func (s *Service) saveStatus(ctx context.Context, status model.JudgeStatusResponse) error {
	ctxStatus := ctx
	if s.statusTimeout > 0 {
		var cancel context.CancelFunc
		ctxStatus, cancel = context.WithTimeout(ctx, s.statusTimeout)
		defer cancel()
	}
	return s.statusRepo.Save(ctxStatus, status)
}

func (s *Service) handleFailure(ctx context.Context, submissionID string, err error) error {
	code := appErr.GetCode(err)
	failed := model.JudgeStatusResponse{
		SubmissionID: submissionID,
		Status:       result.StatusFailed,
		Verdict:      result.VerdictSE,
		ErrorCode:    int(code),
		ErrorMessage: err.Error(),
		Timestamps: result.Timestamps{
			FinishedAt: time.Now().Unix(),
		},
	}
	if saveErr := s.saveStatus(ctx, failed); saveErr != nil {
		logger.Warn(ctx, "update failure status failed", zap.Error(saveErr))
	}
	if code == appErr.InvalidParams || code == appErr.ProblemNotFound || code == appErr.LanguageNotSupported {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return err
}

func resolveLanguageConfig(cfg model.ProblemConfig, languageID string) ([]string, model.ResourceLimit) {
	base := cfg.DefaultLimits
	var extra []string
	for _, lim := range cfg.LanguageLimits {
		if lim.LanguageID == languageID {
			if lim.Limits != nil {
				base = model.MergeLimits(lim.Limits, base)
			}
			extra = append(extra, lim.ExtraCompileFlags...)
			break
		}
	}
	return extra, base
}

func buildTestcases(manifest model.Manifest, basePath string, defaults model.ResourceLimit) ([]sandbox.TestcaseSpec, []sandbox.SubtaskSpec, error) {
	ioCfg := sandbox.IOConfig{
		Mode:           manifest.IOConfig.Mode,
		InputFileName:  manifest.IOConfig.InputFileName,
		OutputFileName: manifest.IOConfig.OutputFileName,
	}

	tests := make([]sandbox.TestcaseSpec, 0, len(manifest.Tests))
	for _, tc := range manifest.Tests {
		inputPath, err := safeJoin(basePath, tc.InputPath)
		if err != nil {
			return nil, nil, err
		}
		answerPath := ""
		if tc.AnswerPath != "" {
			answerPath, err = safeJoin(basePath, tc.AnswerPath)
			if err != nil {
				return nil, nil, err
			}
		}
		limits := model.MergeLimits(tc.Limits, defaults)
		checker := tc.Checker
		if checker == nil {
			checker = manifest.Checker
		}
		var checkerSpec *sandbox.CheckerSpec
		if checker != nil {
			checkerPath, err := safeJoin(basePath, checker.BinaryPath)
			if err != nil {
				return nil, nil, err
			}
			checkerSpec = &sandbox.CheckerSpec{
				BinaryPath: checkerPath,
				Args:       checker.Args,
				Env:        checker.Env,
				Limits:     model.ToSandboxLimit(model.MergeLimits(checker.Limits, defaults)),
			}
		}

		tests = append(tests, sandbox.TestcaseSpec{
			TestID:            tc.TestID,
			InputPath:         inputPath,
			AnswerPath:        answerPath,
			IOConfig:          ioCfg,
			Score:             tc.Score,
			SubtaskID:         tc.SubtaskID,
			Limits:            model.ToSandboxLimit(limits),
			Checker:           checkerSpec,
			CheckerLanguageID: tc.CheckerLanguageID,
		})
	}

	subtasks := make([]sandbox.SubtaskSpec, 0, len(manifest.Subtasks))
	for _, st := range manifest.Subtasks {
		subtasks = append(subtasks, sandbox.SubtaskSpec{
			ID:         st.ID,
			Score:      st.Score,
			Strategy:   st.Strategy,
			StopOnFail: st.StopOnFail,
		})
	}
	return tests, subtasks, nil
}

func safeJoin(basePath, relPath string) (string, error) {
	if relPath == "" {
		return "", appErr.ValidationError("path", "required")
	}
	clean := filepath.Clean(relPath)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", appErr.New(appErr.InvalidParams).WithMessage("invalid relative path")
	}
	full := filepath.Join(basePath, clean)
	if !strings.HasPrefix(full, filepath.Clean(basePath)+string(filepath.Separator)) {
		return "", appErr.New(appErr.InvalidParams).WithMessage("path traversal detected")
	}
	return full, nil
}
