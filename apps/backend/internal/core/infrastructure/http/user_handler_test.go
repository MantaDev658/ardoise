package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"opensplit/apps/backend/internal/core/domain"
	"opensplit/apps/backend/internal/core/mocks"

	"golang.org/x/crypto/bcrypt"
)

func TestAPIHandler_Users(t *testing.T) {
	uRepo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error { return nil },
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
		},
		ListAllFunc: func(ctx context.Context) ([]domain.User, error) {
			return []domain.User{{ID: "Alice", DisplayName: "Alice"}}, nil
		},
	}
	es, us, gs := newTestServices(&mocks.MockExpenseRepo{}, uRepo, &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
	handler := NewAPIHandler(es, us, gs)

	t.Run("GET /users returns list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users", nil)
		rr := httptest.NewRecorder()

		handler.ListUsers(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("Alice")) {
			t.Errorf("expected body to contain Alice, got %s", rr.Body.String())
		}
	})

	t.Run("PUT /users/{id} updates display name", func(t *testing.T) {
		body := []byte(`{"display_name": "Alice Updated"}`)
		req := httptest.NewRequest("PUT", "/users/Alice", bytes.NewBuffer(body))
		req.SetPathValue("id", "Alice")
		rr := httptest.NewRecorder()

		handler.UpdateUser(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("DELETE /users/{id} soft deletes", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/users/Alice", nil)
		req.SetPathValue("id", "Alice")
		rr := httptest.NewRecorder()

		handler.DeleteUser(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestAPIHandler_ChangePassword(t *testing.T) {
	const currentPlain = "password123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(currentPlain), bcrypt.DefaultCost)

	uRepo := &mocks.MockUserRepo{
		GetByIDFunc: func(_ context.Context, _ domain.UserID) (*domain.User, error) {
			return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
		},
	}
	es, us, gs := newTestServices(&mocks.MockExpenseRepo{}, uRepo, &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
	handler := NewAPIHandler(es, us, gs)

	makeReq := func(callerID, targetID, body string) (*httptest.ResponseRecorder, *http.Request) {
		req := httptest.NewRequest("PUT", "/users/"+targetID+"/password", bytes.NewBufferString(body))
		req.SetPathValue("id", targetID)
		ctx := context.WithValue(req.Context(), UserIDKey, callerID)
		return httptest.NewRecorder(), req.WithContext(ctx)
	}

	t.Run("returns 403 when caller is not the target", func(t *testing.T) {
		rr, req := makeReq("Bob", "Alice", `{"current_password":"password123","new_password":"newpassword123"}`)
		handler.ChangePassword(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("returns 400 when current password is wrong", func(t *testing.T) {
		rr, req := makeReq("Alice", "Alice", `{"current_password":"wrong","new_password":"newpassword123"}`)
		handler.ChangePassword(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("returns 400 when new password is too short", func(t *testing.T) {
		rr, req := makeReq("Alice", "Alice", `{"current_password":"password123","new_password":"short"}`)
		handler.ChangePassword(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("returns 400 when new password equals current", func(t *testing.T) {
		rr, req := makeReq("Alice", "Alice", `{"current_password":"password123","new_password":"password123"}`)
		handler.ChangePassword(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("returns 200 on success", func(t *testing.T) {
		rr, req := makeReq("Alice", "Alice", `{"current_password":"password123","new_password":"newpassword123"}`)
		handler.ChangePassword(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

func TestAPIHandler_Auth(t *testing.T) {
	uRepo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error { return nil },
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
		},
	}
	es, us, gs := newTestServices(&mocks.MockExpenseRepo{}, uRepo, &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
	handler := NewAPIHandler(es, us, gs)

	t.Run("POST /auth/register returns 409 on duplicate username", func(t *testing.T) {
		dupRepo := &mocks.MockUserRepo{
			SaveFunc: func(ctx context.Context, user domain.User) error {
				return domain.ErrUserAlreadyExists
			},
		}
		_, dupUS, _ := newTestServices(&mocks.MockExpenseRepo{}, dupRepo, &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
		dupHandler := NewAPIHandler(es, dupUS, gs)

		body := []byte(`{"id": "Alice", "display_name": "Alice", "password": "password123"}`)
		req := httptest.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		dupHandler.RegisterUser(rr, req)

		if rr.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("username already taken")) {
			t.Errorf("expected error message in body, got %s", rr.Body.String())
		}
	})

	t.Run("POST /auth/register creates user", func(t *testing.T) {
		body := []byte(`{"id": "Alice", "display_name": "Alice", "password": "password123"}`)
		req := httptest.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.RegisterUser(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rr.Code)
		}
	})

	t.Run("POST /auth/login returns token", func(t *testing.T) {
		body := []byte(`{"id": "Alice", "password": "password123"}`)
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.LoginUser(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("token")) {
			t.Errorf("expected JSON with token, got %s", rr.Body.String())
		}
	})
}
