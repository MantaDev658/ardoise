package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/infrastructure/http/middleware"
	"ardoise/apps/backend/internal/core/mocks"

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
	})

	t.Run("PUT /users/{id} updates display name", func(t *testing.T) {
		body := []byte(`{"display_name": "Alice Updated"}`)
		req := httptest.NewRequest("PUT", "/users/Alice", bytes.NewBuffer(body))
		req.SetPathValue("id", "Alice")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.UpdateUser(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("PUT /users/{id} returns 403 when caller is not the target", func(t *testing.T) {
		body := []byte(`{"display_name": "Hacked"}`)
		req := httptest.NewRequest("PUT", "/users/Bob", bytes.NewBuffer(body))
		req.SetPathValue("id", "Bob")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.UpdateUser(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("DELETE /users/{id} soft deletes", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/users/Alice", nil)
		req.SetPathValue("id", "Alice")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.DeleteUser(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("DELETE /users/{id} returns 403 when caller is not the target", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/users/Bob", nil)
		req.SetPathValue("id", "Bob")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.DeleteUser(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}

func TestAPIHandler_ChangePassword(t *testing.T) {
	makeUserRepo := func() *mocks.MockUserRepo {
		hash, _ := bcrypt.GenerateFromPassword([]byte("currentpass"), bcrypt.DefaultCost)
		return &mocks.MockUserRepo{
			GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
				return &domain.User{ID: id, IsActive: true, PasswordHash: string(hash)}, nil
			},
			UpdatePasswordFunc: func(ctx context.Context, id domain.UserID, newHash string) error {
				return nil
			},
		}
	}

	makeRequest := func(targetID, callerID, body string) (*httptest.ResponseRecorder, *http.Request) {
		req := httptest.NewRequest("PUT", "/users/"+targetID+"/password", bytes.NewBufferString(body))
		req.SetPathValue("id", targetID)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, callerID)
		req = req.WithContext(ctx)
		return httptest.NewRecorder(), req
	}

	t.Run("returns 403 when caller is not the target", func(t *testing.T) {
		_, us, gs := newTestServices(&mocks.MockExpenseRepo{}, makeUserRepo(), &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
		h := NewAPIHandler(nil, us, gs)
		rr, req := makeRequest("Bob", "Alice", `{"current_password":"currentpass","new_password":"newpass1"}`)
		h.ChangePassword(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("returns 400 when current password is wrong", func(t *testing.T) {
		_, us, gs := newTestServices(&mocks.MockExpenseRepo{}, makeUserRepo(), &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
		h := NewAPIHandler(nil, us, gs)
		rr, req := makeRequest("Alice", "Alice", `{"current_password":"wrongpass","new_password":"newpass123"}`)
		h.ChangePassword(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("returns 400 when new password is too short", func(t *testing.T) {
		_, us, gs := newTestServices(&mocks.MockExpenseRepo{}, makeUserRepo(), &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
		h := NewAPIHandler(nil, us, gs)
		rr, req := makeRequest("Alice", "Alice", `{"current_password":"currentpass","new_password":"short"}`)
		h.ChangePassword(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("returns 400 when new password equals current", func(t *testing.T) {
		_, us, gs := newTestServices(&mocks.MockExpenseRepo{}, makeUserRepo(), &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
		h := NewAPIHandler(nil, us, gs)
		rr, req := makeRequest("Alice", "Alice", `{"current_password":"currentpass","new_password":"currentpass"}`)
		h.ChangePassword(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("returns 200 on success", func(t *testing.T) {
		_, us, gs := newTestServices(&mocks.MockExpenseRepo{}, makeUserRepo(), &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
		h := NewAPIHandler(nil, us, gs)
		rr, req := makeRequest("Alice", "Alice", `{"current_password":"currentpass","new_password":"newpassword123"}`)
		h.ChangePassword(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestGetCurrentUser(t *testing.T) {
	uRepo := &mocks.MockUserRepo{
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			if id == "Alice" {
				return &domain.User{ID: "Alice", DisplayName: "Alice", IsActive: true}, nil
			}
			return nil, domain.ErrUserNotFound
		},
	}
	_, us, gs := newTestServices(&mocks.MockExpenseRepo{}, uRepo, &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
	h := NewAPIHandler(nil, us, gs)

	t.Run("returns 401 when unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/me", nil)
		rr := httptest.NewRecorder()
		h.GetCurrentUser(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("returns current user when authenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/me", nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "Alice"))
		rr := httptest.NewRecorder()
		h.GetCurrentUser(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("Alice")) {
			t.Errorf("expected body to contain Alice, got %s", rr.Body.String())
		}
	})

	t.Run("returns 404 when user not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/me", nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "unknown"))
		rr := httptest.NewRecorder()
		h.GetCurrentUser(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

func TestAPIHandler_ListFriends(t *testing.T) {
	uRepo := &mocks.MockUserRepo{
		ListCoMembersFunc: func(ctx context.Context, userID domain.UserID) ([]domain.User, error) {
			return []domain.User{{ID: "Bob", DisplayName: "Bob"}}, nil
		},
	}
	_, us, gs := newTestServices(&mocks.MockExpenseRepo{}, uRepo, &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
	h := NewAPIHandler(nil, us, gs)

	t.Run("returns 401 when not authenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/friends", nil)
		rr := httptest.NewRecorder()
		h.ListFriends(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("returns friends for authenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/friends", nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "Alice"))
		rr := httptest.NewRecorder()
		h.ListFriends(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestAPIHandler_Auth(t *testing.T) {
	uRepo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error {
			if user.ID == "duplicate" {
				return domain.ErrUserAlreadyExists
			}
			return nil
		},
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			return &domain.User{ID: id, IsActive: true, PasswordHash: string(hash)}, nil
		},
	}
	_, us, gs := newTestServices(&mocks.MockExpenseRepo{}, uRepo, &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
	h := NewAPIHandler(nil, us, gs)

	t.Run("POST /auth/register returns 409 on duplicate username", func(t *testing.T) {
		body := []byte(`{"id":"duplicate","display_name":"Dup","password":"password123"}`)
		req := httptest.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()
		h.RegisterUser(rr, req)
		if rr.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", rr.Code)
		}
	})

	t.Run("POST /auth/register creates user", func(t *testing.T) {
		body := []byte(`{"id":"newuser","display_name":"New User","password":"password123"}`)
		req := httptest.NewRequest("POST", "/auth/register", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()
		h.RegisterUser(rr, req)
		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rr.Code)
		}
	})

	t.Run("POST /auth/login returns token", func(t *testing.T) {
		body := []byte(`{"id":"alice","password":"password123"}`)
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()
		h.LoginUser(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("token")) {
			t.Errorf("expected token in response, got %s", rr.Body.String())
		}
	})
}
