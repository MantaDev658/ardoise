package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/infrastructure/http/middleware"
	"ardoise/apps/backend/internal/core/mocks"
	"ardoise/libs/shared/money"

	"golang.org/x/crypto/bcrypt"
)

func TestAPIHandler_GetBalances(t *testing.T) {
	eRepo := &mocks.MockExpenseRepo{ListForUserFunc: func(ctx context.Context, userID domain.UserID, page domain.Page) ([]*domain.Expense, error) {
		total, _ := money.New(3000)
		split, _ := money.New(1500)
		exp, _ := domain.NewExpense(
			domain.ExpenseID("test-id"),
			nil,
			"Test Dinner",
			total,
			"Alice",
			[]domain.Split{{User: "Alice", Amount: split}, {User: "Bob", Amount: split}},
		)
		return []*domain.Expense{exp}, nil
	}}
	uRepo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error { return nil },
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
		},
	}
	es, us, gs := newTestServices(eRepo, uRepo, &mocks.MockGroupRepo{})
	handler := NewAPIHandler(es, us, gs)

	req, err := http.NewRequest("GET", "/balances", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "Alice"))

	rr := httptest.NewRecorder()
	handler.GetBalances(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expectedBodyFragment := `"suggested_settlements":[{"From":"Bob","To":"Alice","Amount":1500}]`
	if !bytes.Contains(rr.Body.Bytes(), []byte(expectedBodyFragment)) {
		t.Errorf("handler returned unexpected body: got %v", rr.Body.String())
	}
}

func TestAPIHandler_ExpensesGroupFiltering(t *testing.T) {
	es, us, gs := newTestServices(&mocks.MockExpenseRepo{}, &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error { return nil },
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			return &domain.User{ID: id, IsActive: true}, nil
		},
	}, &mocks.MockGroupRepo{})
	handler := NewAPIHandler(es, us, gs)

	t.Run("GET /balances filters by group_id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/balances?group_id=g1", nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "Alice"))
		rr := httptest.NewRecorder()

		handler.GetBalances(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestAPIHandler_Expenses(t *testing.T) {
	eRepo := &mocks.MockExpenseRepo{
		ListForUserFunc: func(ctx context.Context, userID domain.UserID, page domain.Page) ([]*domain.Expense, error) {
			total, _ := money.New(3000)
			split, _ := money.New(1500)
			exp, _ := domain.NewExpense(
				domain.ExpenseID("test-id"),
				nil,
				"Test Dinner",
				total,
				"Alice",
				[]domain.Split{{User: "Alice", Amount: split}, {User: "Bob", Amount: split}},
			)
			return []*domain.Expense{exp}, nil
		},
		GetByIDFunc: func(ctx context.Context, id domain.ExpenseID) (*domain.Expense, error) {
			total, _ := money.New(3000)
			splitAmt, _ := money.New(3000)
			exp, err := domain.NewExpense(id, nil, "Test", total, "Alice", []domain.Split{
				{User: "Alice", Amount: splitAmt},
			})
			if err != nil {
				panic("invalid mock setup: " + err.Error())
			}
			return exp, nil
		},
		DeleteFunc: func(ctx context.Context, id domain.ExpenseID) error { return nil },
	}

	uRepo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error { return nil },
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
		},
	}
	es, us, gs := newTestServices(eRepo, uRepo, &mocks.MockGroupRepo{})
	handler := NewAPIHandler(es, us, gs)

	t.Run("GET /expenses returns list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/expenses", nil)
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "Alice"))
		rr := httptest.NewRecorder()

		handler.ListExpenses(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		expectedBodyFragment := `"description":"Test Dinner","total_cents":3000,"payer":"Alice"`
		if !bytes.Contains(rr.Body.Bytes(), []byte(expectedBodyFragment)) {
			t.Errorf("handler returned unexpected body: got %v", rr.Body.String())
		}
	})

	t.Run("POST /expenses handles invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/expenses", bytes.NewBuffer([]byte("{invalid}")))
		rr := httptest.NewRecorder()

		handler.CreateExpense(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("POST /expenses rejects zero total", func(t *testing.T) {
		body, _ := json.Marshal(application.CreateExpenseCommand{
			TotalCents: 0, Payer: "Alice", SplitType: "EQUAL",
			Splits: []application.SplitDetail{{UserID: "Alice"}},
		})
		req := httptest.NewRequest("POST", "/expenses", bytes.NewBuffer(body))
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.CreateExpense(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for zero total, got %d", rr.Code)
		}
	})

	t.Run("PUT /expenses/{id} successfully updates", func(t *testing.T) {
		cmd := application.UpdateExpenseCommand{
			Description: "Updated Dinner",
			TotalCents:  4000,
			Payer:       "Alice",
			SplitType:   "EQUAL",
			Splits: []application.SplitDetail{
				{UserID: "Alice", Value: 2000},
				{UserID: "Bob", Value: 2000},
			},
		}
		body, _ := json.Marshal(cmd)

		req := httptest.NewRequest("PUT", "/expenses/exp-123", bytes.NewBuffer(body))
		req.SetPathValue("id", "exp-123")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.UpdateExpense(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("DELETE /expenses/{id} successfully deletes", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/expenses/exp-123", nil)
		req.SetPathValue("id", "exp-123")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.DeleteExpense(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestAPIHandler_CreateSettlement(t *testing.T) {
	eRepo := &mocks.MockExpenseRepo{
		SaveFunc: func(ctx context.Context, expense *domain.Expense) error { return nil },
	}
	uRepo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error { return nil },
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
		},
	}
	es, us, gs := newTestServices(eRepo, uRepo, &mocks.MockGroupRepo{})
	handler := NewAPIHandler(es, us, gs)

	t.Run("POST /settlements succeeds", func(t *testing.T) {
		cmd := application.SettleUpCommand{
			PayerID:     "Alice",
			ReceiverID:  "Bob",
			AmountCents: 2000,
		}
		body, _ := json.Marshal(cmd)
		req := httptest.NewRequest("POST", "/settlements", bytes.NewBuffer(body))
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.CreateSettlement(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("POST /settlements handles bad JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/settlements", bytes.NewBufferString("{bad-json}"))
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.CreateSettlement(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rr.Code)
		}
	})
}

func TestAPIHandler_GetFriendBalances(t *testing.T) {
	eRepo := &mocks.MockExpenseRepo{
		GetFriendBalanceSummaryFunc: func(ctx context.Context, userID domain.UserID) ([]domain.FriendBalance, error) {
			return []domain.FriendBalance{
				{FriendID: "Charlie", NetCents: 1000},
			}, nil
		},
	}
	uRepo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error { return nil },
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
			return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
		},
	}
	es, us, gs := newTestServices(eRepo, uRepo, &mocks.MockGroupRepo{})
	handler := NewAPIHandler(es, us, gs)

	t.Run("GET /friends/{user_id}/balances succeeds for auth user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/friends/Alice/balances", nil)
		req.SetPathValue("user_id", "Alice")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.GetFriendBalances(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}

		expectedBody := `"From":"Charlie","To":"Alice","Amount":1000`
		if !bytes.Contains(rr.Body.Bytes(), []byte(expectedBody)) {
			t.Errorf("expected body to contain settlement, got: %s", rr.Body.String())
		}
	})

	t.Run("GET /friends/{user_id}/balances rejects mismatched user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/friends/Bob/balances", nil)
		req.SetPathValue("user_id", "Bob")
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, "Alice")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler.GetFriendBalances(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for mismatched user, got %d", rr.Code)
		}
	})
}
