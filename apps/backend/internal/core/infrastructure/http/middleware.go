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
