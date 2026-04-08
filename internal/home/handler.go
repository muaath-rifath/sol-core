package home

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/muaathrifath/sol-core/internal/user"
)

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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	home, err := h.svc.CreateHome(r.Context(), u.ID, req.Name)
	if err != nil {
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
	homes, err := h.svc.ListHomes(r.Context(), u.ID)
	if err != nil {
		slog.Error("list homes", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, homes)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	home, err := h.svc.UpdateHome(r.Context(), u.ID, id, req.Name)
	if err != nil {
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
	members, err := h.svc.ListMembers(r.Context(), u.ID, id)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		slog.Error("list members", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, members)
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
	member, err := h.svc.AddMember(r.Context(), u.ID, id, req.UserID, role)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
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
	if err := h.svc.UpdateMemberRole(r.Context(), u.ID, homeID, userID, MemberRole(req.Role)); err != nil {
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
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
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
	invitations, err := h.svc.ListInvitations(r.Context(), u.ID, id)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		slog.Error("list invitations", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, invitations)
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
			http.Error(w, `{"error":"invitation not found"}`, http.StatusNotFound)
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

func (h *Handler) DeclineInvitation(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	token := r.PathValue("token")
	if err := h.svc.DeclineInvitation(r.Context(), u.ID, token); err != nil {
		if errors.Is(err, ErrForbidden) {
			http.Error(w, `{"error":"this invitation is not for your email"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"invitation not found"}`, http.StatusNotFound)
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
