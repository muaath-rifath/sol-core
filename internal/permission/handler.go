package permission

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/muaathrifath/sol-core/internal/device"
	"github.com/muaathrifath/sol-core/internal/room"
	"github.com/muaathrifath/sol-core/internal/user"
)

type Handler struct {
	svc       *Service
	roomSvc   *room.Service
	deviceSvc *device.Service
}

func NewHandler(svc *Service, roomSvc *room.Service, deviceSvc *device.Service) *Handler {
	return &Handler{svc: svc, roomSvc: roomSvc, deviceSvc: deviceSvc}
}

// GET /api/v1/homes/{id}/members/{userId}/permissions
func (h *Handler) GetPermissions(w http.ResponseWriter, r *http.Request) {
	caller := user.FromContext(r.Context())
	if caller == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	homeID := r.PathValue("id")
	userID := r.PathValue("userId")
	if homeID == "" || userID == "" {
		http.Error(w, `{"error":"home id and user id are required"}`, http.StatusBadRequest)
		return
	}

	callerRole, err := h.svc.MemberRole(r.Context(), homeID, caller.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		slog.Error("permissions: get caller role", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if callerRole != "owner" && callerRole != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	targetRole, err := h.svc.MemberRole(r.Context(), homeID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"member not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("permissions: get target role", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	tree := PermissionTree{
		HomeID: homeID,
		UserID: userID,
		Role:   targetRole,
	}

	if targetRole == "owner" || targetRole == "admin" {
		tree.AllAccess = true
		tree.Rooms = []TreeRoom{}
		writeJSON(w, http.StatusOK, tree)
		return
	}

	rooms, err := h.roomSvc.ListAllByHome(r.Context(), homeID)
	if err != nil {
		slog.Error("permissions: list rooms", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	roomsSet, devicesSet, appliancesSet, err := h.svc.DirectGrantsFor(r.Context(), homeID, userID)
	if err != nil {
		slog.Error("permissions: list grants", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	managedRoomIDs, err := h.svc.ListManagedRoomIDs(r.Context(), homeID, userID)
	if err != nil {
		slog.Error("permissions: list managed rooms", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	managedSet := make(map[string]struct{}, len(managedRoomIDs))
	for _, id := range managedRoomIDs {
		managedSet[id] = struct{}{}
	}

	out := make([]TreeRoom, 0, len(rooms))
	for _, rm := range rooms {
		treeRoom := TreeRoom{
			ID:               rm.ID,
			Name:             rm.Name,
			GrantedDirectly:  contains(roomsSet, rm.ID),
			CanManageDevices: contains(managedSet, rm.ID),
		}

		devices, err := h.deviceSvc.ListByRoom(r.Context(), rm.ID)
		if err != nil {
			slog.Error("permissions: list devices", "error", err, "room_id", rm.ID)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		treeDevices := make([]TreeDevice, 0, len(devices))
		for _, dev := range devices {
			treeDev := TreeDevice{
				ID:              dev.ID,
				Name:            dev.Name,
				GrantedDirectly: contains(devicesSet, dev.ID),
			}

			apps, err := h.deviceSvc.ListAppliancesByDevice(r.Context(), dev.ID)
			if err != nil {
				slog.Error("permissions: list appliances", "error", err, "device_id", dev.ID)
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}

			treeApps := make([]TreeAppliance, 0, len(apps))
			for _, a := range apps {
				treeApps = append(treeApps, TreeAppliance{
					ID:              a.ID,
					Name:            a.Name,
					Type:            string(a.Type),
					Channel:         a.Channel,
					GrantedDirectly: contains(appliancesSet, a.ID),
				})
			}
			treeDev.Appliances = treeApps
			treeDevices = append(treeDevices, treeDev)
		}
		treeRoom.Devices = treeDevices
		out = append(out, treeRoom)
	}

	tree.Rooms = out
	writeJSON(w, http.StatusOK, tree)
}

// PUT /api/v1/homes/{id}/members/{userId}/permissions
// Body: { "grants": [ { "type": "room"|"device"|"appliance", "id": "..." }, ... ] }
func (h *Handler) PutPermissions(w http.ResponseWriter, r *http.Request) {
	caller := user.FromContext(r.Context())
	if caller == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	homeID := r.PathValue("id")
	userID := r.PathValue("userId")
	if homeID == "" || userID == "" {
		http.Error(w, `{"error":"home id and user id are required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Grants      []ScopeRef `json:"grants"`
		ManageRooms []string   `json:"manage_rooms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := h.svc.SetGrants(r.Context(), caller.ID, homeID, userID, req.Grants); err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		case errors.Is(err, ErrValidation):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, ErrNotFound):
			http.Error(w, `{"error":"member not found"}`, http.StatusNotFound)
		default:
			slog.Error("permissions: set grants", "error", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		}
		return
	}

	manageRooms := req.ManageRooms
	if manageRooms == nil {
		manageRooms = []string{}
	}
	if err := h.svc.SetRoomCapabilities(r.Context(), caller.ID, homeID, userID, manageRooms); err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		case errors.Is(err, ErrValidation):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, ErrNotFound):
			http.Error(w, `{"error":"member not found"}`, http.StatusNotFound)
		default:
			slog.Error("permissions: set room capabilities", "error", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func contains(set map[string]struct{}, id string) bool {
	_, ok := set[id]
	return ok
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
