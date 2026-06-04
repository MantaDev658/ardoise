package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/domain"
)

// POST /groups — creator is taken from JWT, never from request body
func (h *APIHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	cmd, err := validateAndDecode[application.CreateGroupCommand](w, r)
	if err != nil {
		return
	}

	if cmd.CreatorID, err = getAuthUserID(r); err != nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	groupID, err := h.groupService.CreateGroup(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status": "group created", "group_id": "%s"}`, groupID)))
}

// POST /groups/{id}/members
func (h *APIHandler) AddGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")

	cmd, err := decodeJSON[struct {
		UserID string `json:"user_id"`
	}](w, r)
	if err != nil {
		return
	}

	authUserID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.groupService.InviteUserToGroup(r.Context(), groupID, cmd.UserID, authUserID); err != nil {
		switch {
		case errors.Is(err, domain.ErrUnauthorized):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, domain.ErrUserAlreadyInGroup), errors.Is(err, domain.ErrAlreadyInvited):
			http.Error(w, err.Error(), http.StatusConflict)
		case errors.Is(err, domain.ErrUserNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "invitation sent"}`))
}

// GET /groups — returns groups for the authenticated user
func (h *APIHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	userID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	groups, err := h.groupService.ListGroupsForUser(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(groups)
}

// GET /groups/{id}/activity?limit=N&cursor=RFC3339
func (h *APIHandler) GetGroupActivity(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	logs, err := h.expenseService.GetGroupActivity(r.Context(), r.PathValue("id"), page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var nextCursor string
	if page.Limit > 0 && len(logs) == page.Limit {
		nextCursor = logs[len(logs)-1].CreatedAt.UTC().Format(time.RFC3339Nano)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":        logs,
		"next_cursor": nextCursor,
	})
}

// PUT /groups/{id}
func (h *APIHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")

	cmd, err := decodeJSON[struct {
		Name string `json:"name"`
	}](w, r)
	if err != nil {
		return
	}

	authUserID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.groupService.UpdateGroup(r.Context(), groupID, cmd.Name, authUserID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DELETE /groups/{id}
func (h *APIHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")

	authUserID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.groupService.DeleteGroup(r.Context(), groupID, authUserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DELETE /groups/{id}/members/{user_id}
func (h *APIHandler) RemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	userID := r.PathValue("user_id")

	authUserID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.groupService.RemoveMember(r.Context(), groupID, userID, authUserID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
