package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"fuzoj/services/gateway_service/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
)

type BanEvent struct {
	EventType string `json:"event_type"`
	UserID    int64  `json:"user_id"`
	EndTime   string `json:"end_time"`
}

// BanEventHandler consumes ban events and updates local cache.
type BanEventHandler struct {
	banRepo    *repository.BanCacheRepository
	defaultTTL time.Duration
}

func NewBanEventHandler(banRepo *repository.BanCacheRepository, defaultTTL time.Duration) *BanEventHandler {
	return &BanEventHandler{banRepo: banRepo, defaultTTL: defaultTTL}
}

func (h *BanEventHandler) Consume(ctx context.Context, _ string, value string) error {
	if h.banRepo == nil {
		return errors.New("ban repository is nil")
	}
	if value == "" {
		return nil
	}
	var event BanEvent
	if err := json.Unmarshal([]byte(value), &event); err != nil {
		logx.WithContext(ctx).Errorf("parse ban event failed: %v", err)
		return nil
	}
	if event.UserID == 0 {
		return nil
	}

	switch event.EventType {
	case "user.banned":
		ttl := h.defaultTTL
		if event.EndTime != "" {
			if parsed, err := parseEventTime(event.EndTime); err == nil {
				if until := time.Until(parsed); until > 0 {
					ttl = until
				}
			}
		}
		h.banRepo.MarkBanned(event.UserID, ttl)
	case "user.unbanned":
		h.banRepo.UnmarkBanned(event.UserID)
	}
	return nil
}

func parseEventTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("empty time")
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts, nil
	}
	return time.Parse(time.RFC3339, raw)
}
