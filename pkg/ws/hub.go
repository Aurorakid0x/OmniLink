package ws

import (
	"encoding/json"
	"sync"
	"time"

	"OmniLink/pkg/zlog"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Register(c *Client) {
	if c == nil || c.userID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.clients[c.userID]
	if set == nil {
		set = make(map[*Client]struct{})
		h.clients[c.userID] = set
	}
	set[c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	if c == nil || c.userID == "" {
		return
	}
	h.mu.Lock()
	set := h.clients[c.userID]
	if set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, c.userID)
		}
	}
	h.mu.Unlock()
	c.Close()
}

func (h *Hub) Send(userID string, payload []byte) bool {
	if userID == "" || len(payload) == 0 {
		return false
	}

	h.mu.RLock()
	set := h.clients[userID]
	h.mu.RUnlock()
	if len(set) == 0 {
		return false
	}

	ok := false
	for c := range set {
		if c == nil {
			continue
		}
		select {
		case c.send <- payload:
			ok = true
		default:
			h.Unregister(c)
		}
	}
	return ok
}

func (h *Hub) SendJSON(userID string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	h.Send(userID, b)
	return nil
}

type Client struct {
	userID string
	conn   *websocket.Conn
	send   chan []byte

	closeOnce sync.Once
}

func NewClient(userID string, conn *websocket.Conn) *Client {
	return &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, 64),
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.send)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

func (c *Client) WritePump() {
	if c.conn == nil {
		return
	}
	for msg := range c.send {
		_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			zlog.Error(err.Error())
			return
		}
	}
}
