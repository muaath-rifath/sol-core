package voice

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/muaathrifath/sol-core/internal/chat"
	"github.com/muaathrifath/sol-core/internal/user"
)

// ToolDispatcher is satisfied by *chat.Tools.
type ToolDispatcher interface {
	Dispatch(ctx context.Context, name, arguments string, u *user.User, tc chat.ToolContext) string
}

type Handler struct {
	svc   *Service
	tools ToolDispatcher
	pool  *pgxpool.Pool
}

func NewHandler(svc *Service, tools ToolDispatcher, pool *pgxpool.Pool) *Handler {
	return &Handler{svc: svc, tools: tools, pool: pool}
}

// ToolDispatch handles POST /api/internal/voice/tools.
// Called by the Python voice agent to dispatch LLM function calls using the
// device's home-owner context. No user auth required — internal only.
func (h *Handler) ToolDispatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID  string `json:"device_id"`
		Tool      string `json:"tool"`
		Arguments string `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceID == "" || req.Tool == "" {
		http.Error(w, `{"error":"device_id, tool, and arguments required"}`, http.StatusBadRequest)
		return
	}

	// Resolve home_id, room_id, and owner context from the device.
	var homeID, roomID, userID, userName, userEmail string
	err := h.pool.QueryRow(r.Context(), `
		SELECT r.home_id, r.id, u.id, u.name, u.email
		FROM devices d
		JOIN rooms r ON r.id = d.room_id
		JOIN home_members hm ON hm.home_id = r.home_id AND hm.role = 'owner'
		JOIN users u ON u.id = hm.user_id
		WHERE d.id = $1
		LIMIT 1`, req.DeviceID).Scan(&homeID, &roomID, &userID, &userName, &userEmail)
	if err != nil {
		slog.Warn("voice tool dispatch: device context not found", "device_id", req.DeviceID, "error", err)
		http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
		return
	}

	u := &user.User{ID: userID, Name: userName, Email: userEmail}
	result := h.tools.Dispatch(r.Context(), req.Tool, req.Arguments, u, chat.ToolContext{
		HomeID:    homeID,
		RoomID:    roomID,
		ActorType: "esp32",
		ActorID:   req.DeviceID,
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(result))
}

// CreateSession handles POST /api/v1/voice/session.
// Called by the ESP32 over HTTPS when the "Joy" wake word fires.
// Body: {"device_id": "<id>"}
// Response: {"room_name": "...", "token": "...", "url": "..."}
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceID == "" {
		http.Error(w, `{"error":"device_id required"}`, http.StatusBadRequest)
		return
	}

	session, err := h.svc.CreateSession(r.Context(), req.DeviceID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}
