package chat

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/muaathrifath/sol-core/internal/user"
)

// Handler upgrades HTTP connections to WebSocket and starts a Session.
// Only one session per (userID, homeID) pair is allowed at a time; a new
// connection cancels any existing session for that pair.
type Handler struct {
	tools    *Tools
	cfg      SessionConfig
	mu       sync.Mutex
	sessions map[string]context.CancelFunc
}

func NewHandler(tools *Tools, cfg SessionConfig) *Handler {
	return &Handler{
		tools:    tools,
		cfg:      cfg,
		sessions: make(map[string]context.CancelFunc),
	}
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	homeID := r.PathValue("homeId")
	if homeID == "" {
		http.Error(w, "missing homeId", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Warn("chat: websocket accept", "error", err)
		return
	}
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	key := u.ID + ":" + homeID
	h.mu.Lock()
	if prev, ok := h.sessions[key]; ok {
		prev()
	}
	h.sessions[key] = cancel
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		if h.sessions[key] == cancel {
			delete(h.sessions, key)
		}
		h.mu.Unlock()
	}()

	if err := NewSession(conn, u, homeID, h.tools, h.cfg).Run(ctx); err != nil {
		slog.Debug("chat: session ended", "error", err)
	}
}
