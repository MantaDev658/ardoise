package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/libs/shared/money"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func TestExpenseRepository_Lifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewExpenseRepository(db)
	ctx := context.Background()

	expenseID := domain.ExpenseID(uuid.NewString())
	total, _ := money.New(5000)
	split1, _ := money.New(2500)
	split2, _ := money.New(2500)

	exp, err := domain.NewExpense(
		expenseID,
		nil,
		"Integration Test Dinner",
		total,
		"Alice",
		[]domain.Split{
			{User: "Alice", Amount: split1},
			{User: "Bob", Amount: split2},
		},
	)
	if err != nil {
		t.Fatalf("failed to create domain expense: %v", err)
	}

	err = repo.Save(ctx, exp)
	if err != nil {
		t.Fatalf("failed to save expense: %v", err)
	}

	fetchedExp, err := repo.GetByID(ctx, expenseID)
	if err != nil {
		t.Fatalf("failed to get expense by id: %v", err)
	}

	if fetchedExp.ID() != exp.ID() {
		t.Errorf("expected ID %s, got %s", exp.ID(), fetchedExp.ID())
	}
	if fetchedExp.Total().Int64() != 5000 {
		t.Errorf("expected total 5000, got %d", fetchedExp.Total().Int64())
	}
	if len(fetchedExp.Splits()) != 2 {
		t.Errorf("expected 2 splits, got %d", len(fetchedExp.Splits()))
	}

	allExpenses, err := repo.ListAll(ctx, domain.Page{})
	if err != nil {
		t.Fatalf("failed to list all expenses: %v", err)
	}

	if len(allExpenses) != 1 {
		t.Errorf("expected 1 expense in db, got %d", len(allExpenses))
	}
	if allExpenses[0].Description() != "Integration Test Dinner" {
		t.Errorf("expected description 'Integration Test Dinner', got %s", allExpenses[0].Description())
	}
}

func TestExpenseRepository_Pagination_TieBreaker(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewExpenseRepository(db)
	ctx := context.Background()

	total, _ := money.New(1000)
	split, _ := money.New(1000)

	id1 := domain.ExpenseID(uuid.NewString())
	id2 := domain.ExpenseID(uuid.NewString())

	exp1, _ := domain.NewExpense(id1, nil, "First", total, "Alice", []domain.Split{{User: "Alice", Amount: split}})
	exp2, _ := domain.NewExpense(id2, nil, "Second", total, "Alice", []domain.Split{{User: "Alice", Amount: split}})

	if err := repo.Save(ctx, exp1); err != nil {
		t.Fatalf("failed to save exp1: %v", err)
	}
	if err := repo.Save(ctx, exp2); err != nil {
		t.Fatalf("failed to save exp2: %v", err)
	}

	// Force identical created_at so the tie-breaker (id) is the only ordering signal.
	fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`UPDATE expenses SET created_at = $1`, fixedTime); err != nil {
		t.Fatalf("failed to fix timestamps: %v", err)
	}

	page1, err := repo.ListAll(ctx, domain.Page{Limit: 1})
	if err != nil {
		t.Fatalf("page 1 failed: %v", err)
	}
	if len(page1) != 1 {
		t.Fatalf("expected 1 result on page 1, got %d", len(page1))
	}

	page2, err := repo.ListAll(ctx, domain.Page{
		Limit:    1,
		Cursor:   page1[0].CreatedAt(),
		CursorID: string(page1[0].ID()),
	})
	if err != nil {
		t.Fatalf("page 2 failed: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 result on page 2, got %d — tie-breaker cursor is not working", len(page2))
	}
	if page2[0].ID() == page1[0].ID() {
		t.Error("page 2 returned the same expense as page 1; tie-breaker cursor is broken")
	}

	bothIDs := map[domain.ExpenseID]bool{page1[0].ID(): true, page2[0].ID(): true}
	if !bothIDs[id1] || !bothIDs[id2] {
		t.Errorf("paginated results don't cover both expenses: page1=%s page2=%s", page1[0].ID(), page2[0].ID())
	}
}

// TestExpenseRepository_Pagination_MultiSplitNotTruncated guards against the bug
// where LIMIT was applied to the expense×splits join rather than to distinct
// expenses. With multi-split expenses that made LIMIT cut mid-expense, yielding a
// partially-populated expense whose splits no longer summed to the total and so
// failed reconstruction (ErrSplitsDoNotEqualTotal -> "corrupted data in db").
func TestExpenseRepository_Pagination_MultiSplitNotTruncated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewExpenseRepository(db)
	ctx := context.Background()

	total, _ := money.New(1000)
	half, _ := money.New(500)

	// Two expenses, each split between Alice and Bob (2 rows per expense once joined).
	exp1, _ := domain.NewExpense(domain.ExpenseID(uuid.NewString()), nil, "First", total, "Alice",
		[]domain.Split{{User: "Alice", Amount: half}, {User: "Bob", Amount: half}})
	exp2, _ := domain.NewExpense(domain.ExpenseID(uuid.NewString()), nil, "Second", total, "Alice",
		[]domain.Split{{User: "Alice", Amount: half}, {User: "Bob", Amount: half}})

	if err := repo.Save(ctx, exp1); err != nil {
		t.Fatalf("failed to save exp1: %v", err)
	}
	if err := repo.Save(ctx, exp2); err != nil {
		t.Fatalf("failed to save exp2: %v", err)
	}

	// Limit must count expenses, not joined rows: Limit 1 returns 1 complete expense.
	page1, err := repo.ListAll(ctx, domain.Page{Limit: 1})
	if err != nil {
		t.Fatalf("ListAll with Limit 1 failed (expense was truncated mid-split): %v", err)
	}
	if len(page1) != 1 {
		t.Fatalf("expected 1 expense, got %d", len(page1))
	}
	if got := len(page1[0].Splits()); got != 2 {
		t.Fatalf("expected expense to keep all 2 splits, got %d (split set was truncated)", got)
	}

	// Limit 2 returns both expenses (4 joined rows), proving the limit is per-expense.
	page2, err := repo.ListAll(ctx, domain.Page{Limit: 2})
	if err != nil {
		t.Fatalf("ListAll with Limit 2 failed: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 expenses, got %d", len(page2))
	}
}

func TestExpenseRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewExpenseRepository(db)

	randomID := domain.ExpenseID(uuid.NewString())
	_, err := repo.GetByID(context.Background(), randomID)
	if !errors.Is(err, domain.ErrExpenseNotFound) {
		t.Errorf("expected ErrExpenseNotFound, got %v", err)
	}
}
