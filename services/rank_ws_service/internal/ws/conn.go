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

func (c *wsConn) Send(_ context.Context, payload []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, payload)
}

func (c *wsConn) Close() error {
	return c.conn.Close()
}

func NewSender(conn *websocket.Conn) sender {
	return newWSConn(conn)
}
