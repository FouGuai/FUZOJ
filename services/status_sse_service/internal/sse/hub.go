package sse

import (
	"context"
	"errors"
	"sync"
	"time"

	"fuzoj/pkg/submit/statuspubsub"
	"fuzoj/pkg/submit/statuswriter"
	"fuzoj/services/status_sse_service/internal/repository"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	eventSnapshot = "snapshot"
	eventUpdate   = "update"
	eventFinal    = "final"
)

type Hub struct {
	repo       *repository.StatusRepository
	redis      *red.Client
	debounce   time.Duration
	heartbeat  time.Duration
	mu         sync.RWMutex
	subs       map[string]map[*subscription]struct{}
	pubsubs    map[string]*red.PubSub
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func NewHub(repo *repository.StatusRepository, redisClient *red.Client, debounce, heartbeat time.Duration) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	if debounce <= 0 {
		debounce = 100 * time.Millisecond
	}
	if heartbeat <= 0 {
		heartbeat = 15 * time.Second
	}
	return &Hub{
		repo:       repo,
		redis:      redisClient,
		debounce:   debounce,
		heartbeat:  heartbeat,
		subs:       make(map[string]map[*subscription]struct{}),
		pubsubs:    make(map[string]*red.PubSub),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

func (h *Hub) Close() {
	if h == nil {
		return
	}
	h.cancelFunc()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, sub := range h.pubsubs {
		_ = sub.Close()
	}
	h.pubsubs = map[string]*red.PubSub{}
}

func (h *Hub) Subscribe(ctx context.Context, submissionID string, userID int64, include string, sender sender) error {
	if h == nil || h.repo == nil {
		return errors.New("hub is not configured")
	}
	if err := h.repo.CheckSubmissionOwner(ctx, submissionID, userID); err != nil {
		return err
	}
	return h.SubscribeAuthorized(ctx, submissionID, include, sender)
}

func (h *Hub) SubscribeAuthorized(ctx context.Context, submissionID string, include string, sender sender) error {
	if h == nil || h.repo == nil {
		return errors.New("hub is not configured")
	}
	var sub *subscription
	sub = newSubscription(submissionID, include, sender, h.repo, h.debounce, h.heartbeat, func() {
		h.removeSub(submissionID, sub)
	})
	h.addSub(submissionID, sub)
	sub.start(ctx)
	return nil
}

func (h *Hub) addSub(submissionID string, sub *subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[submissionID] == nil {
		h.subs[submissionID] = make(map[*subscription]struct{})
	}
	h.subs[submissionID][sub] = struct{}{}
	h.ensurePubSubLocked(submissionID)
}

func (h *Hub) removeSub(submissionID string, sub *subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs := h.subs[submissionID]; subs != nil {
		delete(subs, sub)
		if len(subs) == 0 {
			delete(h.subs, submissionID)
			if pubsub := h.pubsubs[submissionID]; pubsub != nil {
				_ = pubsub.Close()
			}
			delete(h.pubsubs, submissionID)
		}
	}
}

func (h *Hub) ensurePubSubLocked(submissionID string) {
	if h.redis == nil {
		return
	}
	if _, ok := h.pubsubs[submissionID]; ok {
		return
	}
	channel := statuspubsub.Channel(submissionID)
	pubsub := h.redis.Subscribe(h.ctx, channel)
	h.pubsubs[submissionID] = pubsub
	go h.listenPubSub(submissionID, pubsub)
}

func (h *Hub) listenPubSub(submissionID string, pubsub *red.PubSub) {
	logger := logx.WithContext(h.ctx)
	for {
		msg, err := pubsub.ReceiveMessage(h.ctx)
		if err != nil {
			if errors.Is(err, red.ErrClosed) || h.ctx.Err() != nil {
				return
			}
			if !h.isActivePubSub(submissionID, pubsub) {
				return
			}
			logger.Errorf("status pubsub receive failed: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if msg == nil {
			continue
		}
		h.broadcastRefresh(submissionID)
	}
}

func (h *Hub) isActivePubSub(submissionID string, pubsub *red.PubSub) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	current, ok := h.pubsubs[submissionID]
	return ok && current == pubsub
}

func (h *Hub) broadcastRefresh(submissionID string) {
	h.mu.RLock()
	subsMap := h.subs[submissionID]
	if len(subsMap) == 0 {
		h.mu.RUnlock()
		return
	}
	subs := make([]*subscription, 0, len(subsMap))
	for sub := range subsMap {
		subs = append(subs, sub)
	}
	h.mu.RUnlock()
	for _, sub := range subs {
		sub.notifyRefresh()
	}
}

type subscription struct {
	submissionID string
	include      string
	debounce     time.Duration
	heartbeat    time.Duration
	repo         *repository.StatusRepository
	sender       sender
	refreshCh    chan struct{}
	onClose      func()
}

func newSubscription(submissionID, include string, sender sender, repo *repository.StatusRepository, debounce, heartbeat time.Duration, onClose func()) *subscription {
	return &subscription{
		submissionID: submissionID,
		include:      include,
		debounce:     debounce,
		heartbeat:    heartbeat,
		repo:         repo,
		sender:       sender,
		refreshCh:    make(chan struct{}, 1),
		onClose:      onClose,
	}
}

func (s *subscription) start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *subscription) loop(ctx context.Context) {
	defer func() {
		if s.onClose != nil {
			s.onClose()
		}
	}()
	logger := logx.WithContext(ctx)
	if err := s.sendSnapshot(ctx); err != nil {
		logger.Errorf("send status snapshot failed: %v", err)
		// Keep stream alive with a fallback snapshot so clients do not block forever.
		if fallbackErr := s.sender.Send(eventSnapshot, toMessage(s.submissionID, pendingPayload(s.submissionID))); fallbackErr != nil {
			logger.Errorf("send fallback status snapshot failed: %v", fallbackErr)
			return
		}
	}

	heartbeatTicker := time.NewTicker(s.heartbeat)
	defer heartbeatTicker.Stop()
	var timer *time.Timer

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			if err := s.sender.Ping(); err != nil {
				logger.Errorf("send status ping failed: %v", err)
				return
			}
			// Fallback refresh on heartbeat to avoid stale pending stream
			// when pubsub notification is missed.
			isFinal, err := s.sendUpdate(ctx)
			if err != nil {
				logger.Errorf("send status update on heartbeat failed: %v", err)
				return
			}
			if isFinal {
				return
			}
		case <-s.refreshCh:
			if timer == nil {
				timer = time.NewTimer(s.debounce)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(s.debounce)
			}
		case <-func() <-chan time.Time {
			if timer == nil {
				return make(chan time.Time)
			}
			return timer.C
		}():
			isFinal, err := s.sendUpdate(ctx)
			if err != nil {
				logger.Errorf("send status update failed: %v", err)
				return
			}
			if isFinal {
				return
			}
		}
	}
}

func pendingPayload(submissionID string) statuswriter.StatusPayload {
	return statuswriter.StatusPayload{
		SubmissionID: submissionID,
		Status:       "Pending",
		Progress: statuswriter.Progress{
			TotalTests: 0,
			DoneTests:  0,
		},
	}
}

func (s *subscription) notifyRefresh() {
	select {
	case s.refreshCh <- struct{}{}:
	default:
	}
}

func (s *subscription) sendSnapshot(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	status, err := s.repo.GetLatestStatus(ctx, s.submissionID)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	summary := repository.BuildSummary(status)
	msg := toMessage(s.submissionID, summary)
	return s.sender.Send(eventSnapshot, msg)
}

func (s *subscription) sendUpdate(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	status, err := s.repo.GetLatestStatus(ctx, s.submissionID)
	if err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	summary := repository.BuildSummary(status)
	if err := s.sender.Send(eventUpdate, toMessage(s.submissionID, summary)); err != nil {
		return false, err
	}
	if !repository.IsFinalStatus(status.Status) {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	finalStatus, err := s.repo.GetFinalStatus(ctx, s.submissionID)
	if err != nil {
		finalStatus = status
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := s.sender.Send(eventFinal, toMessage(s.submissionID, finalStatus)); err != nil {
		return false, err
	}
	return true, nil
}

type statusMessage struct {
	SubmissionID string                     `json:"submission_id"`
	EventAt      int64                      `json:"event_at"`
	Data         statuswriter.StatusPayload `json:"data"`
}

func toMessage(submissionID string, status statuswriter.StatusPayload) statusMessage {
	return statusMessage{
		SubmissionID: submissionID,
		EventAt:      time.Now().UnixMilli(),
		Data:         status,
	}
}
