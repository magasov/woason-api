package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"woason-api/internal/models"
)

type Message struct {
	Type    string `json:"type"`
	Channel string `json:"channel,omitempty"`
	PeerID  string `json:"peerId,omitempty"`
	OrderID string `json:"orderId,omitempty"`
	Text    string `json:"text,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

type conn struct {
	user *models.Principal
	ws   *websocket.Conn
	send chan []byte
	subs map[string]struct{}
}

type Hub struct {
	mu     sync.RWMutex
	conns  map[*conn]struct{}
	byUser map[string]map[*conn]struct{}
}

func NewHub() *Hub {
	return &Hub{
		conns:  map[*conn]struct{}{},
		byUser: map[string]map[*conn]struct{}{},
	}
}

func (h *Hub) add(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[c] = struct{}{}
	if h.byUser[c.user.ID] == nil {
		h.byUser[c.user.ID] = map[*conn]struct{}{}
	}
	h.byUser[c.user.ID][c] = struct{}{}
}

func (h *Hub) remove(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, c)
	if m := h.byUser[c.user.ID]; m != nil {
		delete(m, c)
		if len(m) == 0 {
			delete(h.byUser, c.user.ID)
		}
	}
	close(c.send)
}

func (h *Hub) SendToUser(userID string, msg Message) {
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.byUser[userID] {
		select {
		case c.send <- raw:
		default:
		}
	}
}

func (h *Hub) SendOrder(buyerID, sellerID string, msg Message) {
	h.SendToUser(buyerID, msg)
	h.SendToUser(sellerID, msg)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatHandler func(user *models.Principal, peerID, text string) (*models.ChatMessage, string, string, error)

func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, user *models.Principal, onChat ChatHandler) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade", "err", err.Error())
		return
	}
	c := &conn{
		user: user,
		ws:   ws,
		send: make(chan []byte, 32),
		subs: map[string]struct{}{},
	}
	h.add(c)
	go c.write()
	defer func() {
		h.remove(c)
		_ = ws.Close()
	}()

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "subscribe":
			key := msg.Channel + ":" + msg.PeerID + msg.OrderID
			c.subs[key] = struct{}{}
		case "chat.send":
			if onChat == nil || msg.PeerID == "" || msg.Text == "" {
				continue
			}
			saved, sellerID, buyerID, err := onChat(user, msg.PeerID, msg.Text)
			if err != nil {
				raw, _ := json.Marshal(Message{Type: "error", Text: err.Error()})
				c.send <- raw
				continue
			}
			out := Message{Type: "chat.message", PeerID: msg.PeerID, Payload: saved}
			h.SendToUser(sellerID, out)
			h.SendToUser(buyerID, out)
		case "chat.typing":
			h.SendToUser(msg.PeerID, Message{Type: "chat.typing", PeerID: user.ID})
		}
	}
}

func (c *conn) write() {
	for raw := range c.send {
		if err := c.ws.WriteMessage(websocket.TextMessage, raw); err != nil {
			return
		}
	}
}
