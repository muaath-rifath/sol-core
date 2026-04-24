package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"
)

type Hub struct {
	rdb     *redis.Client
	clients map[*conn]bool
	mu      sync.RWMutex
}

type conn struct {
	ws   *websocket.Conn
	send chan []byte
}

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		rdb:     rdb,
		clients: make(map[*conn]bool),
	}
}

func (h *Hub) Run(ctx context.Context) {
	if h.rdb == nil {
		slog.Error("Hub.rdb is nil")
		return
	}
	pubsub := h.rdb.Subscribe(ctx, "sol:events")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			h.broadcastRaw([]byte(msg.Payload))
		}
	}
}

func (h *Hub) Broadcast(eventType string, data any) {
	event := Event{Type: eventType, Data: data}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("marshal broadcast event", "error", err)
		return
	}

	ctx := context.Background()
	h.rdb.Publish(ctx, "sol:events", payload)
}

func (h *Hub) broadcastRaw(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			close(c.send)
			delete(h.clients, c)
		}
	}
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("websocket accept", "error", err)
		return
	}

	c := &conn{
		ws:   wsConn,
		send: make(chan []byte, 256),
	}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	go h.writePump(c)
	h.readPump(r.Context(), c)
}

func (h *Hub) writePump(c *conn) {
	defer c.ws.CloseNow()

	for msg := range c.send {
		if err := c.ws.Write(context.Background(), websocket.MessageText, msg); err != nil {
			return
		}
	}
}

func (h *Hub) readPump(ctx context.Context, c *conn) {
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		c.ws.CloseNow()
	}()

	for {
		_, _, err := c.ws.Read(ctx)
		if err != nil {
			return
		}
		// TODO: handle incoming client messages if needed
	}
}
