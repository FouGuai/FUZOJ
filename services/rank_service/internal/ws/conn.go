package ws

import (
	"context"

	"github.com/gorilla/websocket"
)

type wsConn struct {
	conn *websocket.Conn
}

func newWSConn(conn *websocket.Conn) *wsConn {
	return &wsConn{conn: conn}
}

// NewSender wraps a websocket connection as a sender.
func NewSender(conn *websocket.Conn) sender {
	return newWSConn(conn)
}

func (c *wsConn) Send(_ context.Context, payload []byte) error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.WriteMessage(websocket.TextMessage, payload)
}

func (c *wsConn) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
