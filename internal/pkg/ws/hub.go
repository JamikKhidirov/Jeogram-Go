package ws

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jeogram/messenger/internal/pkg/middleware"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // configure in prod
}

// Outbound is a message pushed to connected clients.
type Outbound struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Client is a single websocket connection for a user.
type Client struct {
	UserID string
	conn   *websocket.Conn
	send   chan []byte
}

// Hub fans out real-time events to connected users. It is the delivery layer
// that the notification service uses after consuming Kafka events.
type Hub struct {
	mu         sync.RWMutex
	clients    map[string][]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMsg
}

type broadcastMsg struct {
	UserIDs []string
	Data    []byte
}

// NewHub creates and returns a Hub. Call Run in a goroutine.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string][]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMsg, 256),
	}
}

// Run starts the hub event loop.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c.UserID] = append(h.clients[c.UserID], c)
			h.mu.Unlock()
			log.Debug().Str("user", c.UserID).Msg("ws client registered")
		case c := <-h.unregister:
			h.mu.Lock()
			cls := h.clients[c.UserID]
			for i, cl := range cls {
				if cl == c {
					h.clients[c.UserID] = append(cls[:i], cls[i+1:]...)
					break
				}
			}
			if len(h.clients[c.UserID]) == 0 {
				delete(h.clients, c.UserID)
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			targets := map[string][]*Client{}
			for _, uid := range msg.UserIDs {
				if cls, ok := h.clients[uid]; ok {
					targets[uid] = cls
				}
			}
			h.mu.RUnlock()
			for _, cls := range targets {
				for _, cl := range cls {
					select {
					case cl.send <- msg.Data:
					default:
						close(cl.send)
					}
				}
			}
		}
	}
}

// OnlineUserIDs returns the set of currently connected user IDs.
func (h *Hub) OnlineUserIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for uid := range h.clients {
		ids = append(ids, uid)
	}
	return ids
}

// IsOnline reports whether the user has at least one active connection.
func (h *Hub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

// SendToUsers delivers a payload to all connections of the given users.
func (h *Hub) SendToUsers(userIDs []string, payload Outbound) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.broadcast <- broadcastMsg{UserIDs: userIDs, Data: data}
}

// Handler upgrades the connection and registers the user with the hub.
func (h *Hub) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserID(r)
		if userID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error().Err(err).Msg("ws upgrade failed")
			return
		}
		client := &Client{UserID: userID, conn: conn, send: make(chan []byte, 64)}
		h.register <- client
		go client.writePump()
		go client.readPump(h)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(1 << 20)
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

// Route registers the websocket endpoint under /ws.
func (h *Hub) Route() http.HandlerFunc {
	return h.Handler()
}
