package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/infrastructure/http/middleware"
	"ardoise/apps/backend/internal/core/mocks"

	"golang.org/x/crypto/bcrypt"
)

func TestAPIHandler_Groups(t *testing.T) {
	eRepo := &mocks.MockExpenseRepo{
		ListByGroupFunc: func(ctx context.Context, groupID domain.GroupID, page domain.Page) ([]*domain.Expense, error) {
			return []*domain.Expense{}, nil
		},
	}

	gRepo := &mocks.MockGroupRepo{
		ListForUserFunc: func(ctx context.Context, userID domain.UserID) ([]*domain.Group, error) {
			return []*domain.Group{
				{ID: "g1", Name: "Ski Trip 2026", Members: []domain.UserID{"Alice"}},
			}, nil
		},
		GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
			return &domain.Group{ID: "g1", Name: "Ski Trip 2026", Members: []domain.UserID{"Alice"}}, nil
		},
		UpdateNameFunc:   func(ctx context.Context, id domain.GroupID, newName string) error { return nil },
		DeleteFunc:       func(ctx context.Context, id domain.GroupID) error { return nil },
		RemoveMemberFunc: func(ctx context.Context, groupID domain.GroupID, userID domain.UserID) error { return nil },
		SaveFunc:         func(ctx context.Context, group *domain.Group) error { return nil },
	}
	uRepo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error { return nil },
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
		},
	}
	es, us, gs := newTestServices(eRepo, uRepo, gRepo)
	handler := NewAPIHandler(es, us, gs)

	t.Run("POST /groups creates a group using JWT identity as creator", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"name": "Ski Trip 2026"})
		req := httptest.NewRequest("POST", "/groups", bytes.NewBuffer(body))
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.CreateGroup(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("POST /groups/{id}/members adds a member", func(t *testing.T) {
		body := []byte(`{"user_id": "Bob"}`)
		req := httptest.NewRequest("POST", "/groups/g1/members", bytes.NewBuffer(body))
		req.SetPathValue("id", "g1")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.AddGroupMember(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("GET /groups returns list for authenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/groups", nil)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.ListGroups(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !bytes.Contains(rr.Body.Bytes(), []byte("Ski Trip 2026")) {
			t.Errorf("expected body to contain 'Ski Trip 2026', got %s", rr.Body.String())
		}
	})

	t.Run("GET /groups rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/groups", nil)
		rr := httptest.NewRecorder()

		handler.ListGroups(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("PUT /groups/{id} updates group name", func(t *testing.T) {
		body := []byte(`{"name": "New Trip Name"}`)
		req := httptest.NewRequest("PUT", "/groups/g1", bytes.NewBuffer(body))
		req.SetPathValue("id", "g1")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.UpdateGroup(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("DELETE /groups/{id} deletes group", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/groups/g1", nil)
		req.SetPathValue("id", "g1")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.DeleteGroup(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("DELETE /groups/{id}/members/{user_id} removes member", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/groups/g1/members/Bob", nil)
		req.SetPathValue("id", "g1")
		req.SetPathValue("user_id", "Bob")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.RemoveGroupMember(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}
