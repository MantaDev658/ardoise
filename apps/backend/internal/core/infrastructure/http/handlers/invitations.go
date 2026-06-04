package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"ardoise/apps/backend/internal/core/domain"
)

// GET /invitations — lists pending invitations for the authenticated user
func (h *APIHandler) ListMyInvitations(w http.ResponseWriter, r *http.Request) {
	userID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
		return
	}

	invitations, err := h.groupService.GetMyInvitations(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if invitations == nil {
		invitations = []domain.Invitation{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(invitations)
}

// POST /invitations/{id}/accept
func (h *APIHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	invitationID := r.PathValue("id")

	userID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.groupService.AcceptInvitation(r.Context(), invitationID, userID); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvitationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, domain.ErrUnauthorized):
			http.Error(w, err.Error(), http.StatusForbidden)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// POST /invitations/{id}/decline
func (h *APIHandler) DeclineInvitation(w http.ResponseWriter, r *http.Request) {
	invitationID := r.PathValue("id")

	userID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.groupService.DeclineInvitation(r.Context(), invitationID, userID); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvitationNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, domain.ErrUnauthorized):
			http.Error(w, err.Error(), http.StatusForbidden)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}
