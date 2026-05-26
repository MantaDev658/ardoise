package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/domain"
	hmacauth "ardoise/apps/backend/internal/core/infrastructure/auth/hmac"
	"ardoise/apps/backend/internal/core/mocks"
	sharedjwt "ardoise/libs/shared/jwt"
)

func TestAuthMiddleware(t *testing.T) {
	secret := []byte("test-secret")
	middleware := AuthMiddleware(hmacauth.New(secret))

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

	handlerToTest := middleware(nextHandler)

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

func TestUserProvisioningMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	reqWithUserID := func(id string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		return r.WithContext(context.WithValue(r.Context(), UserIDKey, id))
	}

	t.Run("returns 401 when no UserID in context", func(t *testing.T) {
		svc := application.NewUserService(&mocks.MockUserRepo{}, []byte("s"))
		mw := UserProvisioningMiddleware(svc)(next)

		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("passes through when user already exists", func(t *testing.T) {
		repo := &mocks.MockUserRepo{
			GetByIDFunc: func(_ context.Context, _ domain.UserID) (*domain.User, error) {
				return &domain.User{ID: "user_abc"}, nil
			},
		}
		svc := application.NewUserService(repo, []byte("s"))
		mw := UserProvisioningMiddleware(svc)(next)

		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, reqWithUserID("user_abc"))

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("creates user and passes through when not found", func(t *testing.T) {
		var savedID domain.UserID
		repo := &mocks.MockUserRepo{
			GetByIDFunc: func(_ context.Context, _ domain.UserID) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			SaveFunc: func(_ context.Context, u domain.User) error {
				savedID = u.ID
				return nil
			},
		}
		svc := application.NewUserService(repo, []byte("s"))
		mw := UserProvisioningMiddleware(svc)(next)

		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, reqWithUserID("user_new"))

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if savedID != "user_new" {
			t.Errorf("expected user_new to be saved, got %q", savedID)
		}
	})

	t.Run("returns 500 on repo error during provisioning", func(t *testing.T) {
		repo := &mocks.MockUserRepo{
			GetByIDFunc: func(_ context.Context, _ domain.UserID) (*domain.User, error) {
				return nil, fmt.Errorf("db down")
			},
		}
		svc := application.NewUserService(repo, []byte("s"))
		mw := UserProvisioningMiddleware(svc)(next)

		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, reqWithUserID("user_abc"))

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rr.Code)
		}
	})
}
