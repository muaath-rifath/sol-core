package room

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/muaathrifath/sol-core/internal/user"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	homeID := r.PathValue("homeId")
	cursor := r.URL.Query().Get("cursor")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rooms, err := h.svc.List(r.Context(), homeID, cursor, limit)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, "invalid cursor")
			return
		}
		slog.Error("list rooms", "error", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if rooms.Data == nil {
		rooms.Data = []Room{}
	}
	writeJSON(w, http.StatusOK, rooms)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	homeID := r.PathValue("homeId")
	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusUnprocessableEntity, "name is required")
		return
	}
	room, err := h.svc.Create(r.Context(), homeID, req)
	if err != nil {
		slog.Error("create room", "error", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	room, err := h.svc.Get(r.Context(), roomID, homeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	var req UpdateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	room, err := h.svc.Update(r.Context(), roomID, homeID, req)
	if err != nil {
		slog.Error("update room", "error", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	if err := h.svc.Delete(r.Context(), roomID, homeID); err != nil {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (h *Handler) ListActivity(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	homeID := r.PathValue("homeId")
	roomID := r.PathValue("roomId")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	logs, err := h.svc.ListActivityLogs(r.Context(), roomID, homeID, limit)
	if err != nil {
		slog.Error("list activity logs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": logs})
}
