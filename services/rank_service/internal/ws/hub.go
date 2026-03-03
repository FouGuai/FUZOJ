package ws

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"fuzoj/services/rank_service/internal/repository"
	"fuzoj/services/rank_service/internal/types"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	refreshMsgType  = "refresh"
	snapshotMsgType = "snapshot"
)

// Hub manages websocket subscriptions.
type Hub struct {
	repo       *repository.LeaderboardRepository
	redis      *red.Client
	debounce   time.Duration
	mu         sync.RWMutex
	subs       map[string]map[*subscription]struct{}
	pubsubs    map[string]*red.PubSub
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewHub creates a new websocket hub.
func NewHub(repo *repository.LeaderboardRepository, redisClient *red.Client, debounce time.Duration) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	if debounce <= 0 {
		debounce = 100 * time.Millisecond
	}
	return &Hub{
		repo:       repo,
		redis:      redisClient,
		debounce:   debounce,
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

// Subscribe registers a new websocket subscription.
func (h *Hub) Subscribe(ctx context.Context, contestID string, page, pageSize int, mode string, sender sender) {
	sub := newSubscription(contestID, page, pageSize, mode, sender, h.repo, h.debounce, func() {
		h.removeSub(contestID, sub)
	})
	h.addSub(contestID, sub)
	sub.start(ctx)
}

func (h *Hub) addSub(contestID string, sub *subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[contestID] == nil {
		h.subs[contestID] = make(map[*subscription]struct{})
	}
	h.subs[contestID][sub] = struct{}{}
	h.ensurePubSubLocked(contestID)
}

func (h *Hub) removeSub(contestID string, sub *subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs := h.subs[contestID]; subs != nil {
		delete(subs, sub)
		if len(subs) == 0 {
			delete(h.subs, contestID)
			if pubsub := h.pubsubs[contestID]; pubsub != nil {
				_ = pubsub.Close()
			}
			delete(h.pubsubs, contestID)
		}
	}
}

func (h *Hub) ensurePubSubLocked(contestID string) {
	if h.redis == nil {
		return
	}
	if _, ok := h.pubsubs[contestID]; ok {
		return
	}
	channel := pubsubChannel(contestID)
	pubsub := h.redis.Subscribe(h.ctx, channel)
	h.pubsubs[contestID] = pubsub
	go h.listenPubSub(contestID, pubsub)
}

func (h *Hub) listenPubSub(contestID string, pubsub *red.PubSub) {
	logger := logx.WithContext(h.ctx)
	for {
		msg, err := pubsub.ReceiveMessage(h.ctx)
		if err != nil {
			if errors.Is(err, red.ErrClosed) {
				return
			}
			if h.ctx.Err() != nil {
				return
			}
			if !h.isActivePubSub(contestID, pubsub) {
				return
			}
			logger.Errorf("rank pubsub receive failed: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if msg == nil {
			continue
		}
		h.broadcastRefresh(contestID)
	}
}

func (h *Hub) broadcastRefresh(contestID string) {
	h.mu.RLock()
	subsMap := h.subs[contestID]
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

func (h *Hub) isActivePubSub(contestID string, pubsub *red.PubSub) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	current, ok := h.pubsubs[contestID]
	return ok && current == pubsub
}

func pubsubChannel(contestID string) string {
	return "contest:lb:pubsub:" + contestID
}

type sender interface {
	Send(ctx context.Context, payload []byte) error
	Close() error
}

type subscription struct {
	contestID string
	page      int
	pageSize  int
	mode      string
	debounce  time.Duration
	repo      *repository.LeaderboardRepository
	sender    sender
	refreshCh chan struct{}
	stopCh    chan struct{}
	onClose   func()
}

func newSubscription(contestID string, page, pageSize int, mode string, sender sender, repo *repository.LeaderboardRepository, debounce time.Duration, onClose func()) *subscription {
	return &subscription{
		contestID: contestID,
		page:      page,
		pageSize:  pageSize,
		mode:      mode,
		debounce:  debounce,
		repo:      repo,
		sender:    sender,
		refreshCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		onClose:   onClose,
	}
}

func (s *subscription) start(ctx context.Context) {
	logger := logx.WithContext(ctx)
	if err := s.sendSnapshot(ctx); err != nil {
		logger.Errorf("send rank snapshot failed: %v", err)
	}
	go s.loop(ctx)
}

func (s *subscription) loop(ctx context.Context) {
	defer func() {
		_ = s.sender.Close()
		if s.onClose != nil {
			s.onClose()
		}
	}()
	var timer *time.Timer
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
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
			_ = s.sendRefresh(ctx)
		}
	}
}

func (s *subscription) notifyRefresh() {
	select {
	case s.refreshCh <- struct{}{}:
	default:
	}
}

func (s *subscription) sendSnapshot(ctx context.Context) error {
	payload, err := s.repo.GetPage(ctx, s.contestID, s.page, s.pageSize, s.mode)
	if err != nil {
		return err
	}
	return s.sendMessage(ctx, snapshotMsgType, payload)
}

func (s *subscription) sendRefresh(ctx context.Context) error {
	payload, err := s.repo.GetPage(ctx, s.contestID, s.page, s.pageSize, s.mode)
	if err != nil {
		return err
	}
	return s.sendMessage(ctx, refreshMsgType, payload)
}

func (s *subscription) sendMessage(ctx context.Context, msgType string, payload types.LeaderboardPayload) error {
	data := map[string]any{
		"type": msgType,
		"data": payload,
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.sender.Send(ctx, bytes)
}
