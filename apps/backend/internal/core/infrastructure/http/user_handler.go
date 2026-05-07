package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"opensplit/apps/backend/internal/core/domain"
)

// POST /auth/register
func (h *APIHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	cmd, err := decodeJSON[struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}](w, r)
	if err != nil {
		return
	}

	if err := h.userService.RegisterUser(r.Context(), cmd.ID, cmd.DisplayName, cmd.Password); err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// POST /auth/login
func (h *APIHandler) LoginUser(w http.ResponseWriter, r *http.Request) {
	cmd, err := decodeJSON[struct {
		ID       string `json:"id"`
		Password string `json:"password"`
	}](w, r)
	if err != nil {
		return
	}

	token, err := h.userService.LoginUser(r.Context(), cmd.ID, cmd.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"token": token}); err != nil {
		return
	}
}

// GET /users
func (h *APIHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userService.ListUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {
		return
	}
}

// PUT /users/{id}
func (h *APIHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	cmd, err := decodeJSON[struct {
		DisplayName string `json:"display_name"`
	}](w, r)
	if err != nil {
		return
	}

	if err := h.userService.UpdateUser(r.Context(), userID, cmd.DisplayName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DELETE /users/{id}
func (h *APIHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if err := h.userService.DeleteUser(r.Context(), userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// PUT /users/{id}/password
func (h *APIHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")

	// Enforce self-only: the caller may only change their own password.
	callerID, err := getAuthUserID(r)
	if err != nil || callerID != targetID {
		http.Error(w, domain.ErrUnauthorized.Error(), http.StatusForbidden)
		return
	}

	cmd, err := decodeJSON[struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}](w, r)
	if err != nil {
		return
	}

	if err := h.userService.ChangePassword(r.Context(), targetID, cmd.CurrentPassword, cmd.NewPassword); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			// 400 not 401: caller is authenticated, this is a validation failure not session expiry
			http.Error(w, "current password is incorrect", http.StatusBadRequest)
		case errors.Is(err, domain.ErrPasswordTooShort), errors.Is(err, domain.ErrSamePassword):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}
