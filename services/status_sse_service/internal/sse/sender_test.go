package sse

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type panicFlushWriter struct {
	header http.Header
}

func newPanicFlushWriter() *panicFlushWriter {
	return &panicFlushWriter{header: make(http.Header)}
}

func (w *panicFlushWriter) Header() http.Header {
	return w.header
}

func (w *panicFlushWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (w *panicFlushWriter) WriteHeader(statusCode int) {}

func (w *panicFlushWriter) Flush() {
	panic("flush panic for test")
}

func TestSSESenderSendWritesSSEFrame(t *testing.T) {
	recorder := httptest.NewRecorder()
	sender := NewSender(recorder, recorder)

	if err := sender.Send("update", map[string]string{"status": "finished"}); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "event: update\n") {
		t.Fatalf("event line not found, body=%q", body)
	}
	if !strings.Contains(body, "data: ") {
		t.Fatalf("data line not found, body=%q", body)
	}
}

func TestSSESenderSendRecoversFlushPanic(t *testing.T) {
	writer := newPanicFlushWriter()
	sender := NewSender(writer, writer)

	err := sender.Send("update", map[string]string{"status": "finished"})
	if err == nil {
		t.Fatal("expected send error when flusher panics")
	}
	if !errors.Is(err, errSSEFlushPanic) {
		t.Fatalf("expected errSSEFlushPanic, got %v", err)
	}
}

func TestSSESenderPingRecoversFlushPanic(t *testing.T) {
	writer := newPanicFlushWriter()
	sender := NewSender(writer, writer)

	err := sender.Ping()
	if err == nil {
		t.Fatal("expected ping error when flusher panics")
	}
	if !errors.Is(err, errSSEFlushPanic) {
		t.Fatalf("expected errSSEFlushPanic, got %v", err)
	}
}
