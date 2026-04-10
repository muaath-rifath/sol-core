package home

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/muaathrifath/sol-core/internal/user"
)

// parseCursorLimit reads ?cursor= and ?limit= query params.
// Missing/invalid limit falls back to service defaults.
func parseCursorLimit(r *http.Request) (cursor string, limit int) {
	cursor = r.URL.Query().Get("cursor")
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	return
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) CreateHome(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	home, err := h.svc.CreateHome(r.Context(), u.ID, req.Name)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		slog.Error("create home", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, home)
}

func (h *Handler) ListHomes(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	cursor, limit := parseCursorLimit(r)
	resp, err := h.svc.ListHomes(r.Context(), u.ID, cursor, limit)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("list homes", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetHome(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	home, err := h.svc.GetHome(r.Context(), u.ID, id)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, home)
}

func (h *Handler) UpdateHome(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	home, err := h.svc.UpdateHome(r.Context(), u.ID, id, req.Name)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("update home", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, home)
}

func (h *Handler) DeleteHome(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	if err := h.svc.DeleteHome(r.Context(), u.ID, id); err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		slog.Error("delete home", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	cursor, limit := parseCursorLimit(r)
	resp, err := h.svc.ListMembers(r.Context(), u.ID, id, cursor, limit)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("list members", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}
	role := MemberRole(req.Role)
	if role == "" {
		role = RoleMember
	}
	if role != RoleMember && role != RoleAdmin {
		writeError(w, http.StatusBadRequest, "role must be 'admin' or 'member'")
		return
	}
	member, err := h.svc.AddMember(r.Context(), u.ID, id, req.UserID, role)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		slog.Error("add member", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	homeID := r.PathValue("id")
	userID := r.PathValue("userId")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Role == "" {
		http.Error(w, `{"error":"role is required"}`, http.StatusBadRequest)
		return
	}
	role := MemberRole(req.Role)
	if role != RoleMember && role != RoleAdmin {
		writeError(w, http.StatusBadRequest, "role must be 'admin' or 'member' (use transfer-ownership to change the owner)")
		return
	}
	if err := h.svc.UpdateMemberRole(r.Context(), u.ID, homeID, userID, role); err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("update member role", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	homeID := r.PathValue("id")
	userID := r.PathValue("userId")
	if err := h.svc.RemoveMember(r.Context(), u.ID, homeID, userID); err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		slog.Error("remove member", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TransferOwnership hands home ownership from the caller to another member.
// The caller must be the current owner.
func (h *Handler) TransferOwnership(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	homeID := r.PathValue("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.TransferOwnership(r.Context(), u.ID, homeID, req.UserID); err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"target user is not a member of this home"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("transfer ownership", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) InviteByEmail(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, `{"error":"email is required"}`, http.StatusBadRequest)
		return
	}
	inv, err := h.svc.InviteByEmail(r.Context(), u.ID, id, req.Email)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		slog.Error("invite by email", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

func (h *Handler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	statusFilter := r.URL.Query().Get("status")
	cursor, limit := parseCursorLimit(r)
	resp, err := h.svc.ListInvitations(r.Context(), u.ID, id, statusFilter, cursor, limit)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("list invitations", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CancelInvitation(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	homeID := r.PathValue("id")
	invID := r.PathValue("invId")
	if err := h.svc.CancelInvitation(r.Context(), u.ID, homeID, invID); err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrConflict) {
			http.Error(w, `{"error":"invitation is not pending"}`, http.StatusConflict)
			return
		}
		slog.Error("cancel invitation", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetInvitation returns public details about an invitation — no auth required.
// The frontend uses this to render the invite landing page before the user logs in.
func (h *Handler) GetInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	detail, err := h.svc.GetInvitation(r.Context(), token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"invitation not found or expired"}`, http.StatusNotFound)
			return
		}
		slog.Error("get invitation", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	token := r.PathValue("token")
	home, err := h.svc.AcceptInvitation(r.Context(), u.ID, token)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"this invitation is not for your email"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"invitation not found or expired"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrConflict) {
			http.Error(w, `{"error":"invitation already used or expired"}`, http.StatusConflict)
			return
		}
		slog.Error("accept invitation", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, home)
}

// DeclineInvitation declines an invitation by token — no auth required.
// The token itself (256-bit secret) is sufficient proof of intent.
// This allows non-registered users to decline without creating an account.
func (h *Handler) DeclineInvitation(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if err := h.svc.DeclineInvitation(r.Context(), token); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"invitation not found or expired"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrConflict) {
			http.Error(w, `{"error":"invitation already used or expired"}`, http.StatusConflict)
			return
		}
		slog.Error("decline invitation", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response with the given status and message.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
