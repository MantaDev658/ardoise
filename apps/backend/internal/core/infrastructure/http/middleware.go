package http

import (
	"context"
	"net/http"
	"strings"

	"ardoise/apps/backend/internal/core/application"
)

type contextKey string

// UserIDKey is the context key under which AuthMiddleware stores the authenticated user's ID.
const UserIDKey contextKey = "user_id"

// AuthMiddleware extracts the Bearer token from each request, delegates verification
// to the provided Authenticator, and injects the resolved UserID into the context.
func AuthMiddleware(auth application.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error": "unauthorized missing token"}`, http.StatusUnauthorized)
				return
			}
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := auth.Authenticate(r.Context(), tokenString)
			if err != nil {
				http.Error(w, `{"error": "invalid token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, string(userID))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type displayNameContextKey string

const displayNameKey displayNameContextKey = "display_name"

// WithDisplayName attaches a display name to the request context so that
// UserProvisioningMiddleware can use it when creating the user record.
func WithDisplayName(r *http.Request, name string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), displayNameKey, name))
}

// UserProvisioningMiddleware auto-creates a user record on first authenticated request
// when using an external identity provider (e.g. Clerk). It is a no-op if the user
// already exists. Must run after AuthMiddleware.
func UserProvisioningMiddleware(svc *application.UserService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(UserIDKey).(string)
			if !ok || userID == "" {
				http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
				return
			}
			displayName, _ := r.Context().Value(displayNameKey).(string)
			if displayName == "" {
				displayName = userID
			}
			if err := svc.ProvisionUser(r.Context(), userID, displayName); err != nil {
				http.Error(w, `{"error": "user provisioning failed"}`, http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
