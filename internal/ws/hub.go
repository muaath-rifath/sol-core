package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/muaathrifath/sol-core/internal/user"
	"github.com/redis/go-redis/v9"
)

type Hub struct {
	rdb            *redis.Client
	clients        map[*conn]bool
	mu             sync.RWMutex
	commandHandler CommandHandlerFunc
}

// CommandHandlerFunc is called for each message the client sends over the WS connection.
type CommandHandlerFunc func(ctx context.Context, u *user.User, msg ClientMessage) error

// ClientMessage is the envelope for all client-to-server WS messages.
type ClientMessage struct {
	Type          string          `json:"type"`
	CorrelationID string          `json:"correlationId,omitempty"`
	Data          json.RawMessage `json:"data"`
}

type conn struct {
	ws   *websocket.Conn
	send chan []byte
	user *user.User
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

// SetCommandHandler registers the function that handles incoming client messages.
func (h *Hub) SetCommandHandler(fn CommandHandlerFunc) {
	h.commandHandler = fn
}

func (h *Hub) Run(ctx context.Context) {
	slog.Info("Hub.Run started", "h", h)
	if h == nil {
		slog.Error("Hub is nil")
		return
	}
	if h.rdb == nil {
		slog.Error("Hub.rdb is nil")
		return
	}
	slog.Info("Hub.rdb found", "rdb", h.rdb)
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
		user: user.FromContext(r.Context()),
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

func (h *Hub) sendToConn(c *conn, eventType string, data any) {
	payload, err := json.Marshal(Event{Type: eventType, Data: data})
	if err != nil {
		return
	}
	select {
	case c.send <- payload:
	default:
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
		_, raw, err := c.ws.Read(ctx)
		if err != nil {
			return
		}

		if h.commandHandler == nil {
			continue
		}

		var msg ClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			slog.Warn("ws: invalid client message", "error", err)
			continue
		}

		if err := h.commandHandler(ctx, c.user, msg); err != nil {
			slog.Warn("ws: command handler error", "error", err, "type", msg.Type)
			h.sendToConn(c, "command.ack", map[string]any{
				"correlationId": msg.CorrelationID,
				"success":       false,
				"error":         err.Error(),
			})
			continue
		}

		h.sendToConn(c, "command.ack", map[string]any{
			"correlationId": msg.CorrelationID,
			"success":       true,
			"error":         nil,
		})
	}
}
