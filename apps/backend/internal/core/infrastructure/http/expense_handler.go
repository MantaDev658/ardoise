package http

import (
	"encoding/json"
	"net/http"
	"time"

	"opensplit/apps/backend/internal/core/application"
	"opensplit/apps/backend/internal/core/domain"
)

// POST /expenses
func (h *APIHandler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	cmd, err := validateAndDecode[application.CreateExpenseCommand](w, r)
	if err != nil {
		return
	}

	if cmd.Payer, err = getAuthUserID(r); err != nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := h.expenseService.AddExpense(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// GET /expenses?group_id={optional}&limit=N&cursor=RFC3339
func (h *APIHandler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	groupID := r.URL.Query().Get("group_id")

	userID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var expenses []*domain.Expense

	if groupID != "" {
		expenses, err = h.expenseService.ListExpensesByGroup(r.Context(), groupID, page)
	} else {
		expenses, err = h.expenseService.ListExpensesForUser(r.Context(), userID, page)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type expenseItem struct {
		ID          string    `json:"id"`
		Description string    `json:"description"`
		Total       int64     `json:"total_cents"`
		Payer       string    `json:"payer"`
		CreatedAt   time.Time `json:"created_at"`
	}

	items := make([]expenseItem, len(expenses))
	for i, exp := range expenses {
		items[i] = expenseItem{
			ID:          string(exp.ID()),
			Description: exp.Description(),
			Total:       exp.Total().Int64(),
			Payer:       string(exp.Payer()),
			CreatedAt:   exp.CreatedAt(),
		}
	}

	var nextCursor string
	if page.Limit > 0 && len(expenses) == page.Limit {
		nextCursor = expenses[len(expenses)-1].CreatedAt().UTC().Format(time.RFC3339Nano)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"data":        items,
		"next_cursor": nextCursor,
	}); err != nil {
		return
	}
}

// GET /balances?group_id={optional}
func (h *APIHandler) GetBalances(w http.ResponseWriter, r *http.Request) {
	groupID := r.URL.Query().Get("group_id")

	userID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var expenses []*domain.Expense

	if groupID != "" {
		expenses, err = h.expenseService.ListExpensesByGroup(r.Context(), groupID, domain.Page{})
	} else {
		expenses, err = h.expenseService.ListExpensesForUser(r.Context(), userID, domain.Page{})
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balances := domain.CalculateNetBalances(expenses)
	suggestions := domain.SimplifyDebts(balances)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"net_balances":          balances,
		"suggested_settlements": suggestions,
	}); err != nil {
		return
	}
}

// GET /friends/{user_id}/balances
func (h *APIHandler) GetFriendBalances(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	authUserID, err := getAuthUserID(r)
	if err != nil || authUserID != userID {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	debts, err := h.expenseService.GetFriendBalances(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(debts); err != nil {
		return
	}
}

// PUT /expenses/{id}
func (h *APIHandler) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	expenseID := r.PathValue("id")

	cmd, err := validateAndDecode[application.UpdateExpenseCommand](w, r)
	if err != nil {
		return
	}
	cmd.ID = expenseID

	if err := h.expenseService.UpdateExpense(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "expense updated"}`))
}

// DELETE /expenses/{id}
func (h *APIHandler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	expenseID := r.PathValue("id")

	authUserID, err := getAuthUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := h.expenseService.DeleteExpense(r.Context(), expenseID, authUserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "expense deleted"}`))
}

// POST /settlements
func (h *APIHandler) CreateSettlement(w http.ResponseWriter, r *http.Request) {
	cmd, err := decodeJSON[application.SettleUpCommand](w, r)
	if err != nil {
		return
	}

	if cmd.PayerID, err = getAuthUserID(r); err != nil {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := h.expenseService.SettleUp(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
