package voice

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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
