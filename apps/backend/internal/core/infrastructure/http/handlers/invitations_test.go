package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/infrastructure/http/middleware"
	"ardoise/apps/backend/internal/core/mocks"
)

// newHandlerWithInvitationRepo builds an APIHandler whose GroupService uses
// a custom invitation repo, letting invitation-handler tests control service behaviour.
func newHandlerWithInvitationRepo(invRepo *mocks.MockInvitationRepo) *APIHandler {
	tx := &mocks.MockTransactor{}
	gRepo := &mocks.MockGroupRepo{
		GetByIDFunc: func(_ context.Context, id domain.GroupID) (*domain.Group, error) {
			return &domain.Group{ID: id, Name: "Trip", Members: []domain.UserID{"Alice"}}, nil
		},
	}
	gs := application.NewGroupService(gRepo, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, invRepo, &mocks.MockUserRepo{}, tx)
	es := application.NewExpenseService(&mocks.MockExpenseRepo{}, gRepo, &mocks.MockAuditRepo{}, tx)
	us := application.NewUserService(&mocks.MockUserRepo{}, []byte("test-secret"))
	return NewAPIHandler(es, us, gs)
}

func authedRequest(method, target string, userID string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
}

func TestInvitationHandler_ListMyInvitations(t *testing.T) {
	t.Run("returns 200 with invitation list", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetPendingForUserFunc: func(_ context.Context, userID domain.UserID) ([]domain.Invitation, error) {
				return []domain.Invitation{
					{ID: "inv-1", GroupID: "g1", GroupName: "Trip", InviterID: "Alice", InviteeID: "Bob"},
				}, nil
			},
		}
		h := newHandlerWithInvitationRepo(invRepo)

		rr := httptest.NewRecorder()
		h.ListMyInvitations(rr, authedRequest("GET", "/invitations", "Bob"))

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		var out []map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(out) != 1 {
			t.Errorf("expected 1 invitation, got %d", len(out))
		}
	})

	t.Run("returns 200 with empty array when no invitations", func(t *testing.T) {
		h := newHandlerWithInvitationRepo(&mocks.MockInvitationRepo{})

		rr := httptest.NewRecorder()
		h.ListMyInvitations(rr, authedRequest("GET", "/invitations", "Bob"))

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		var out []any
		if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(out) != 0 {
			t.Errorf("expected empty array, got %d items", len(out))
		}
	})

	t.Run("returns 401 when unauthenticated", func(t *testing.T) {
		h := newHandlerWithInvitationRepo(&mocks.MockInvitationRepo{})

		rr := httptest.NewRecorder()
		h.ListMyInvitations(rr, httptest.NewRequest("GET", "/invitations", nil))

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})
}

func TestInvitationHandler_AcceptInvitation(t *testing.T) {
	t.Run("returns 200 on success", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, id string) (domain.Invitation, error) {
				return domain.Invitation{ID: id, GroupID: "g1", InviterID: "Alice", InviteeID: "Bob"}, nil
			},
		}
		h := newHandlerWithInvitationRepo(invRepo)

		req := authedRequest("POST", "/invitations/inv-1/accept", "Bob")
		req.SetPathValue("id", "inv-1")
		rr := httptest.NewRecorder()
		h.AcceptInvitation(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("returns 401 when unauthenticated", func(t *testing.T) {
		h := newHandlerWithInvitationRepo(&mocks.MockInvitationRepo{})

		req := httptest.NewRequest("POST", "/invitations/inv-1/accept", nil)
		req.SetPathValue("id", "inv-1")
		rr := httptest.NewRecorder()
		h.AcceptInvitation(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("returns 403 when actor is not the invitee", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, id string) (domain.Invitation, error) {
				return domain.Invitation{ID: id, GroupID: "g1", InviterID: "Alice", InviteeID: "Bob"}, nil
			},
		}
		h := newHandlerWithInvitationRepo(invRepo)

		req := authedRequest("POST", "/invitations/inv-1/accept", "Mallory")
		req.SetPathValue("id", "inv-1")
		rr := httptest.NewRecorder()
		h.AcceptInvitation(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("returns 404 when invitation not found", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, _ string) (domain.Invitation, error) {
				return domain.Invitation{}, domain.ErrInvitationNotFound
			},
		}
		h := newHandlerWithInvitationRepo(invRepo)

		req := authedRequest("POST", "/invitations/missing/accept", "Bob")
		req.SetPathValue("id", "missing")
		rr := httptest.NewRecorder()
		h.AcceptInvitation(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

func TestInvitationHandler_DeclineInvitation(t *testing.T) {
	t.Run("returns 200 on success", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, id string) (domain.Invitation, error) {
				return domain.Invitation{ID: id, GroupID: "g1", InviterID: "Alice", InviteeID: "Bob"}, nil
			},
		}
		h := newHandlerWithInvitationRepo(invRepo)

		req := authedRequest("POST", "/invitations/inv-1/decline", "Bob")
		req.SetPathValue("id", "inv-1")
		rr := httptest.NewRecorder()
		h.DeclineInvitation(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("returns 401 when unauthenticated", func(t *testing.T) {
		h := newHandlerWithInvitationRepo(&mocks.MockInvitationRepo{})

		req := httptest.NewRequest("POST", "/invitations/inv-1/decline", nil)
		req.SetPathValue("id", "inv-1")
		rr := httptest.NewRecorder()
		h.DeclineInvitation(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("returns 403 when actor is not the invitee", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, id string) (domain.Invitation, error) {
				return domain.Invitation{ID: id, GroupID: "g1", InviterID: "Alice", InviteeID: "Bob"}, nil
			},
		}
		h := newHandlerWithInvitationRepo(invRepo)

		req := authedRequest("POST", "/invitations/inv-1/decline", "Mallory")
		req.SetPathValue("id", "inv-1")
		rr := httptest.NewRecorder()
		h.DeclineInvitation(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("returns 404 when invitation not found", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, _ string) (domain.Invitation, error) {
				return domain.Invitation{}, domain.ErrInvitationNotFound
			},
		}
		h := newHandlerWithInvitationRepo(invRepo)

		req := authedRequest("POST", "/invitations/missing/decline", "Bob")
		req.SetPathValue("id", "missing")
		rr := httptest.NewRecorder()
		h.DeclineInvitation(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

// Verify the handler maps unknown service errors to 500, not a panic or wrong code.
func TestInvitationHandler_ServiceErrors_Return500(t *testing.T) {
	serviceErr := errors.New("database is on fire")

	for _, tc := range []struct {
		name    string
		handler func(h *APIHandler, rr *httptest.ResponseRecorder, req *http.Request)
	}{
		{
			name: "AcceptInvitation",
			handler: func(h *APIHandler, rr *httptest.ResponseRecorder, req *http.Request) {
				req.SetPathValue("id", "inv-1")
				h.AcceptInvitation(rr, req)
			},
		},
		{
			name: "DeclineInvitation",
			handler: func(h *APIHandler, rr *httptest.ResponseRecorder, req *http.Request) {
				req.SetPathValue("id", "inv-1")
				h.DeclineInvitation(rr, req)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invRepo := &mocks.MockInvitationRepo{
				GetByIDFunc: func(_ context.Context, _ string) (domain.Invitation, error) {
					return domain.Invitation{}, serviceErr
				},
			}
			h := newHandlerWithInvitationRepo(invRepo)

			rr := httptest.NewRecorder()
			req := authedRequest("POST", "/invitations/inv-1/action", "Bob")
			tc.handler(h, rr, req)

			if rr.Code != http.StatusInternalServerError {
				t.Errorf("expected 500, got %d", rr.Code)
			}
		})
	}
}
