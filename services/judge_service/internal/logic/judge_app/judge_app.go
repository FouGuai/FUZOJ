package judge_app

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"fuzoj/internal/common/storage"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/cache"
	pmodel "fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/problemclient"
	"fuzoj/services/judge_service/internal/repository"
	"fuzoj/services/judge_service/internal/sandbox"
	"fuzoj/services/judge_service/internal/sandbox/result"
)

// JudgeApp handles judge tasks.
type JudgeApp struct {
	worker         *sandbox.Worker
	statusRepo     *repository.StatusRepository
	problemClient  *problemclient.Client
	dataCache      *cache.DataPackCache
	storage        storage.ObjectStorage
	retryPusher    MessagePusher
	deadPusher     MessagePusher
	sourceBucket   string
	workRoot       string
	workerTimeout  time.Duration
	problemTimeout time.Duration
	storageTimeout time.Duration
	statusTimeout  time.Duration
	metaTTL        time.Duration
	retryTopic     string
	poolRetryMax   int
	poolRetryBase  time.Duration
	poolRetryMaxD  time.Duration
	deadLetter     string
	sem            chan struct{}

	metaMu    sync.Mutex
	metaCache map[int64]metaEntry
}

type metaEntry struct {
	meta      pmodel.ProblemMeta
	expiresAt time.Time
}

// JudgeAppConfig holds processor dependencies and settings.
type JudgeAppConfig struct {
	Worker         *sandbox.Worker
	StatusRepo     *repository.StatusRepository
	ProblemClient  *problemclient.Client
	DataCache      *cache.DataPackCache
	Storage        storage.ObjectStorage
	RetryPusher    MessagePusher
	DeadPusher     MessagePusher
	SourceBucket   string
	WorkRoot       string
	WorkerTimeout  time.Duration
	ProblemTimeout time.Duration
	StorageTimeout time.Duration
	StatusTimeout  time.Duration
	MetaTTL        time.Duration
	WorkerPoolSize int
	RetryTopic     string
	PoolRetryMax   int
	PoolRetryBase  time.Duration
	PoolRetryMaxD  time.Duration
	DeadLetter     string
}

// NewJudgeApp creates a new judge processor.
func NewJudgeApp(cfg JudgeAppConfig) (*JudgeApp, error) {
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
	svc := &JudgeApp{
		worker:         cfg.Worker,
		statusRepo:     cfg.StatusRepo,
		problemClient:  cfg.ProblemClient,
		dataCache:      cfg.DataCache,
		storage:        cfg.Storage,
		retryPusher:    cfg.RetryPusher,
		deadPusher:     cfg.DeadPusher,
		sourceBucket:   cfg.SourceBucket,
		workRoot:       cfg.WorkRoot,
		workerTimeout:  cfg.WorkerTimeout,
		problemTimeout: cfg.ProblemTimeout,
		storageTimeout: cfg.StorageTimeout,
		statusTimeout:  cfg.StatusTimeout,
		metaTTL:        cfg.MetaTTL,
		retryTopic:     cfg.RetryTopic,
		poolRetryMax:   cfg.PoolRetryMax,
		poolRetryBase:  cfg.PoolRetryBase,
		poolRetryMaxD:  cfg.PoolRetryMaxD,
		deadLetter:     cfg.DeadLetter,
		sem:            make(chan struct{}, poolSize),
		metaCache:      make(map[int64]metaEntry),
	}
	if svc.worker != nil {
		svc.worker.SetStatusReporter(svc)
	}
	return svc, nil
}

// HandleMessage processes a judge task message.
func (s *JudgeApp) HandleMessage(ctx context.Context, payload pmodel.JudgeMessage) error {
	if payload.SubmissionID == "" || payload.ProblemID <= 0 || payload.LanguageID == "" || payload.SourceKey == "" {
		return appErr.New(appErr.InvalidParams).WithMessage("message missing required fields")
	}

	if s.statusRepo != nil {
		existing, err := s.statusRepo.Get(ctx, payload.SubmissionID)
		if err == nil {
			if statusRank(existing.Status) > statusRank(result.StatusPending) {
				return appErr.New(appErr.InvalidParams).WithMessage("submission status is ahead of incoming message")
			}
		} else if !appErr.Is(err, appErr.NotFound) {
			return err
		}
	}

	now := time.Now().Unix()
	pending := pmodel.JudgeStatusResponse{
		SubmissionID: payload.SubmissionID,
		Status:       result.StatusPending,
		Timestamps:   result.Timestamps{ReceivedAt: now},
		Progress:     pmodel.Progress{TotalTests: 0, DoneTests: 0},
	}
	if err := s.persistStatus(ctx, pending); err != nil {
		return err
	}

	if !s.tryAcquireSlot() {
		if err := s.requeueForPoolFull(ctx, payload); err != nil {
			return err
		}
		return nil
	}
	defer s.releaseSlot()

	meta, err := s.getProblemMeta(ctx, payload.ProblemID)
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, err)
	}
	dataPath, err := s.dataCache.Get(ctx, meta)
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, err)
	}

	manifest, err := pmodel.LoadManifest(filepath.Join(dataPath, "manifest.json"))
	if err != nil {
		return s.handleFailure(ctx, payload.SubmissionID, appErr.Wrapf(err, appErr.JudgeSystemError, "load manifest failed"))
	}
	config, err := pmodel.LoadProblemConfig(filepath.Join(dataPath, "config.json"))
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
		ReceivedAt:        pending.Timestamps.ReceivedAt,
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

	finished := pmodel.JudgeStatusResponse{
		SubmissionID: payload.SubmissionID,
		Status:       res.Status,
		Verdict:      res.Verdict,
		Score:        res.Summary.TotalScore,
		Language:     res.Language,
		Compile:      res.Compile,
		Tests:        res.Tests,
		Summary:      res.Summary,
		Timestamps: result.Timestamps{
			ReceivedAt: pending.Timestamps.ReceivedAt,
			FinishedAt: time.Now().Unix(),
		},
		Progress: pmodel.Progress{TotalTests: len(res.Tests), DoneTests: len(res.Tests)},
	}
	if err := s.persistStatus(ctx, finished); err != nil {
		return err
	}
	return nil
}

func statusRank(status result.JudgeStatus) int {
	switch status {
	case result.StatusPending:
		return 1
	case result.StatusCompiling:
		return 2
	case result.StatusRunning:
		return 3
	case result.StatusJudging:
		return 4
	case result.StatusFinished:
		return 5
	case result.StatusFailed:
		return 6
	default:
		return 0
	}
}
