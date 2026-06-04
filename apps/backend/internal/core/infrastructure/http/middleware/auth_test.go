package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	hmacauth "ardoise/apps/backend/internal/core/infrastructure/auth/hmac"
	sharedjwt "ardoise/libs/shared/jwt"
)

func TestAuthMiddleware(t *testing.T) {
	secret := []byte("test-secret")
	mw := AuthMiddleware(hmacauth.New(secret))

	// panics if the user_id is not injected into the context correctly
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok || userID == "" {
			t.Errorf("expected UserID in context, got none")
		}
		if userID != "Alice" {
			t.Errorf("expected UserID 'Alice', got %s", userID)
		}
		w.WriteHeader(http.StatusOK)
	})

	handlerToTest := mw(nextHandler)

	t.Run("Rejects missing Authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Rejects malformed token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.string")
		rr := httptest.NewRecorder()
		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Rejects token signed with unexpected algorithm (alg:none)", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"sub": "Alice",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		rr := httptest.NewRecorder()
		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Succeeds and injects context with valid token", func(t *testing.T) {
		tokenString, err := sharedjwt.Sign("Alice", secret)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}
