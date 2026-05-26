package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/domain"
)

type APIHandler struct {
	expenseService *application.ExpenseService
	userService    *application.UserService
	groupService   *application.GroupService
}

func NewAPIHandler(es *application.ExpenseService, us *application.UserService, gs *application.GroupService) *APIHandler {
	return &APIHandler{
		expenseService: es,
		userService:    us,
		groupService:   gs,
	}
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var body T
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return body, err
	}
	return body, nil
}

// validateAndDecode decodes the request body into T and calls T.Validate().
// T must implement interface{ Validate() error }.
func validateAndDecode[T interface{ Validate() error }](w http.ResponseWriter, r *http.Request) (T, error) {
	cmd, err := decodeJSON[T](w, r)
	if err != nil {
		return cmd, err
	}
	if err := cmd.Validate(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return cmd, err
	}
	return cmd, nil
}

func getAuthUserID(r *http.Request) (string, error) {
	id, ok := r.Context().Value(UserIDKey).(string)
	if !ok || id == "" {
		return "", domain.ErrUnauthorized
	}
	return id, nil
}

// parsePage reads optional ?limit=N&cursor=<timestamp>|<id> query params.
// The cursor is a composite of RFC3339Nano timestamp and expense ID (tie-breaker).
// Defaults: limit=20, no cursor (first page).
func parsePage(r *http.Request) domain.Page {
	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	var cursor time.Time
	var cursorID string
	if s := r.URL.Query().Get("cursor"); s != "" {
		if idx := strings.Index(s, "|"); idx != -1 {
			if t, err := time.Parse(time.RFC3339Nano, s[:idx]); err == nil {
				cursor = t
				cursorID = s[idx+1:]
			}
		} else {
			// Fallback: bare timestamp (no tie-breaker; safe — just less precise)
			if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
				cursor = t
			}
		}
	}
	return domain.Page{Limit: limit, Cursor: cursor, CursorID: cursorID}
}
