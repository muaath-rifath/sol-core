package room

import (
	"encoding/json"
	"log/slog"
	"net/http"
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
	rooms, err := h.svc.List(r.Context(), homeID)
	if err != nil {
		slog.Error("list rooms", "error", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if rooms == nil {
		rooms = []Room{}
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
