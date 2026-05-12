package chat

import (
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/muaathrifath/sol-core/internal/user"
)

// Handler upgrades HTTP connections to WebSocket and starts a Session.
type Handler struct {
	tools *Tools
	cfg   SessionConfig
}

func NewHandler(tools *Tools, cfg SessionConfig) *Handler {
	return &Handler{tools: tools, cfg: cfg}
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

	if err := NewSession(conn, u, homeID, h.tools, h.cfg).Run(r.Context()); err != nil {
		slog.Debug("chat: session ended", "error", err)
	}
}
