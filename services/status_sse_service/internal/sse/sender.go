package sse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

var errSSEFlushPanic = errors.New("sse flush panic")

type sender interface {
	Send(event string, payload any) error
	Ping() error
}

type sseSender struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

func NewSender(w http.ResponseWriter, flusher http.Flusher) sender {
	return &sseSender{w: w, flusher: flusher}
}

func (s *sseSender) Send(event string, payload any) error {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", string(bytes)); err != nil {
		return err
	}
	return safeFlush(s.flusher)
}

func (s *sseSender) Ping() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprint(s.w, ": ping\n\n"); err != nil {
		return err
	}
	return safeFlush(s.flusher)
}

func safeFlush(flusher http.Flusher) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("flush failed: %w, recovered=%v", errSSEFlushPanic, recovered)
		}
	}()
	flusher.Flush()
	return nil
}
