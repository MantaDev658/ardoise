package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/mocks"
	"ardoise/libs/shared/money"
)

func newTestExpenseService(eRepo *mocks.MockExpenseRepo, gRepo *mocks.MockGroupRepo, aRepo *mocks.MockAuditRepo) *ExpenseService {
	return NewExpenseService(eRepo, gRepo, aRepo, &mocks.MockTransactor{})
}

func TestExpenseService_AddExpense_CoreMath(t *testing.T) {
	tests := []struct {
		name          string
		cmd           CreateExpenseCommand
		expectedError error
	}{
		{
			name: "Path 1: Success EXACT",
			cmd: CreateExpenseCommand{
				TotalCents: 3000, Payer: "Alice", SplitType: "EXACT",
				Splits: []SplitDetail{{UserID: "Alice", Value: 1500}, {UserID: "Bob", Value: 1500}},
			},
			expectedError: nil,
		},
		{
			name: "Path 2: Success EQUAL",
			cmd: CreateExpenseCommand{
				TotalCents: 3000, Payer: "Alice", SplitType: "EQUAL",
				Splits: []SplitDetail{{UserID: "Alice"}, {UserID: "Bob"}},
			},
			expectedError: nil,
		},
		{
			name: "Path 3: Success PERCENTAGE",
			cmd: CreateExpenseCommand{
				TotalCents: 3000, Payer: "Alice", SplitType: "PERCENTAGE",
				Splits: []SplitDetail{{UserID: "Alice", Value: 60.00}, {UserID: "Bob", Value: 40.00}},
			},
			expectedError: nil,
		},
		{
			name: "Path 4: Success SHARES",
			cmd: CreateExpenseCommand{
				TotalCents: 3000, Payer: "Alice", SplitType: "SHARES",
				Splits: []SplitDetail{{UserID: "Alice", Value: 2}, {UserID: "Bob", Value: 1}},
			},
			expectedError: nil,
		},
		{
			name: "Path 5: Math Mismatch EXACT",
			cmd: CreateExpenseCommand{
				TotalCents: 3000, Payer: "Alice", SplitType: "EXACT",
				Splits: []SplitDetail{{UserID: "Alice", Value: 1000}, {UserID: "Bob", Value: 1000}},
			},
			expectedError: domain.ErrSplitsDoNotEqualTotal,
		},
		{
			name: "Path 6: Invalid Split Type",
			cmd: CreateExpenseCommand{
				TotalCents: 3000, Payer: "Alice", SplitType: "INVALID_TYPE",
				Splits: []SplitDetail{{UserID: "Alice", Value: 1000}},
			},
			expectedError: errors.New("unknown allocation strategy"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aRepo := &mocks.MockAuditRepo{}
			eRepo := &mocks.MockExpenseRepo{
				SaveFunc: func(ctx context.Context, expense *domain.Expense) error { return nil },
			}
			service := newTestExpenseService(eRepo, &mocks.MockGroupRepo{}, aRepo)

			err := service.AddExpense(context.Background(), tt.cmd)

			if tt.expectedError == nil && err != nil {
				t.Errorf("expected success, got error: %v", err)
			}
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError.Error())
				} else if !errors.Is(err, tt.expectedError) && err.Error() != "business rule violation: allocation math error: "+tt.expectedError.Error() {
					t.Errorf("expected specific error, got: %v", err)
				}
			}
		})
	}
}

func TestExpenseService_AddExpense_GroupValidation(t *testing.T) {
	aRepo := &mocks.MockAuditRepo{}
	eRepo := &mocks.MockExpenseRepo{
		SaveFunc: func(ctx context.Context, expense *domain.Expense) error { return nil },
	}

	t.Run("Path 7: Fails if Group lookup errors", func(t *testing.T) {
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
				return nil, errors.New("database connection lost")
			},
		}
		service := newTestExpenseService(eRepo, gRepo, aRepo)
		cmd := CreateExpenseCommand{GroupID: "g1", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT", Splits: []SplitDetail{{UserID: "Alice", Value: 3000}}}

		err := service.AddExpense(context.Background(), cmd)
		if err == nil {
			t.Error("expected error due to group DB failure, got nil")
		}
	})

	t.Run("Path 8: Fails if Payer is not in group", func(t *testing.T) {
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Members: []domain.UserID{"Bob", "Charlie"}}, nil
			},
		}
		service := newTestExpenseService(eRepo, gRepo, aRepo)
		cmd := CreateExpenseCommand{GroupID: "g1", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT", Splits: []SplitDetail{{UserID: "Alice", Value: 3000}}}

		err := service.AddExpense(context.Background(), cmd)
		if !errors.Is(err, domain.ErrUserNotInGroup) {
			t.Errorf("expected ErrUserNotInGroup, got %v", err)
		}
	})

	t.Run("Path 9: Fails if Split Participant is not in group", func(t *testing.T) {
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Members: []domain.UserID{"Alice", "Bob"}}, nil
			},
		}
		service := newTestExpenseService(eRepo, gRepo, aRepo)
		cmd := CreateExpenseCommand{GroupID: "g1", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT", Splits: []SplitDetail{{UserID: "Alice", Value: 1500}, {UserID: "David", Value: 1500}}}

		err := service.AddExpense(context.Background(), cmd)
		if err == nil {
			t.Error("expected error for invalid participant, got nil")
		}
	})
}

func TestExpenseService_AddExpense_Infrastructure(t *testing.T) {
	t.Run("Path 10: Fails if DB Save fails", func(t *testing.T) {
		aRepo := &mocks.MockAuditRepo{}
		eRepo := &mocks.MockExpenseRepo{
			SaveFunc: func(ctx context.Context, expense *domain.Expense) error {
				return errors.New("insert failed")
			},
		}
		service := newTestExpenseService(eRepo, &mocks.MockGroupRepo{}, aRepo)
		cmd := CreateExpenseCommand{TotalCents: 3000, Payer: "Alice", SplitType: "EXACT", Splits: []SplitDetail{{UserID: "Alice", Value: 3000}}}

		err := service.AddExpense(context.Background(), cmd)
		if err == nil {
			t.Error("expected infrastructure failure, got nil")
		}
	})
}

func TestExpenseService_AddExpense_AuditIsAtomic(t *testing.T) {
	t.Run("Audit failure rolls back expense save", func(t *testing.T) {
		saved := false
		eRepo := &mocks.MockExpenseRepo{
			SaveFunc: func(ctx context.Context, expense *domain.Expense) error {
				saved = true
				return nil
			},
		}
		aRepo := &mocks.MockAuditRepo{
			SaveFunc: func(ctx context.Context, log domain.AuditLog) error {
				return errors.New("audit DB error")
			},
		}
		// Use a transactor that actually runs fn — but with real rollback semantics,
		// the mock just verifies both calls happen inside the same fn.
		txCalled := false
		tx := &mocks.MockTransactor{
			RunInTxFunc: func(ctx context.Context, fn func(context.Context) error) error {
				txCalled = true
				return fn(ctx)
			},
		}
		service := NewExpenseService(eRepo, &mocks.MockGroupRepo{}, aRepo, tx)
		cmd := CreateExpenseCommand{
			GroupID: "g1", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT",
			Splits: []SplitDetail{{UserID: "Alice", Value: 3000}},
		}
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Members: []domain.UserID{"Alice"}}, nil
			},
		}
		service.groupRepo = gRepo

		err := service.AddExpense(context.Background(), cmd)

		if err == nil {
			t.Error("expected error from audit failure, got nil")
		}
		if !txCalled {
			t.Error("expected RunInTx to be called")
		}
		if !saved {
			t.Error("expected expense Save to be called within transaction")
		}
	})
}

func TestExpenseService_AddExpense_GroupLock(t *testing.T) {
	t.Run("acquires group lock for group expenses", func(t *testing.T) {
		lockCalled := false
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(_ context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Members: []domain.UserID{"Alice"}}, nil
			},
			LockGroupFunc: func(_ context.Context, _ domain.GroupID) error {
				lockCalled = true
				return nil
			},
		}
		service := newTestExpenseService(&mocks.MockExpenseRepo{}, gRepo, &mocks.MockAuditRepo{})
		cmd := CreateExpenseCommand{
			GroupID: "g1", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT",
			Splits: []SplitDetail{{UserID: "Alice", Value: 3000}},
		}

		if err := service.AddExpense(context.Background(), cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !lockCalled {
			t.Error("expected LockGroup to be called for a group expense")
		}
	})

	t.Run("does not acquire group lock for non-group expenses", func(t *testing.T) {
		lockCalled := false
		gRepo := &mocks.MockGroupRepo{
			LockGroupFunc: func(_ context.Context, _ domain.GroupID) error {
				lockCalled = true
				return nil
			},
		}
		service := newTestExpenseService(&mocks.MockExpenseRepo{}, gRepo, &mocks.MockAuditRepo{})
		cmd := CreateExpenseCommand{
			TotalCents: 3000, Payer: "Alice", SplitType: "EXACT",
			Splits: []SplitDetail{{UserID: "Alice", Value: 3000}},
		}

		if err := service.AddExpense(context.Background(), cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lockCalled {
			t.Error("expected LockGroup NOT to be called for a non-group expense")
		}
	})
}

func TestExpenseService_UpdateExpense(t *testing.T) {
	makeOldExpense := func(groupID string) *domain.Expense {
		total, _ := money.New(3000)
		split, _ := money.New(3000)
		var gPtr *domain.GroupID
		if groupID != "" {
			g := domain.GroupID(groupID)
			gPtr = &g
		}
		exp, _ := domain.NewExpenseFromDB(
			"exp-1", gPtr, "Old Desc", total, "Alice",
			[]domain.Split{{User: "Alice", Amount: split}},
			time.Now(),
		)
		return exp
	}

	t.Run("returns error when expense not found", func(t *testing.T) {
		service := newTestExpenseService(&mocks.MockExpenseRepo{}, &mocks.MockGroupRepo{}, &mocks.MockAuditRepo{})
		cmd := UpdateExpenseCommand{ID: "missing", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT", Splits: []SplitDetail{{UserID: "Alice", Value: 3000}}}
		if err := service.UpdateExpense(context.Background(), cmd); !errors.Is(err, domain.ErrExpenseNotFound) {
			t.Errorf("expected ErrExpenseNotFound, got %v", err)
		}
	})

	t.Run("basic update in same group writes one audit save", func(t *testing.T) {
		auditSaves := 0
		eRepo := &mocks.MockExpenseRepo{
			GetByIDFunc: func(_ context.Context, _ domain.ExpenseID) (*domain.Expense, error) {
				return makeOldExpense("group-a"), nil
			},
		}
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(_ context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Members: []domain.UserID{"Alice"}}, nil
			},
		}
		aRepo := &mocks.MockAuditRepo{
			SaveFunc: func(_ context.Context, log domain.AuditLog) error {
				auditSaves++
				if log.Action != domain.AuditActionUpdatedExpense {
					t.Errorf("expected UpdatedExpense audit, got %s", log.Action)
				}
				return nil
			},
		}
		service := newTestExpenseService(eRepo, gRepo, aRepo)
		cmd := UpdateExpenseCommand{ID: "exp-1", GroupID: "group-a", Description: "Updated", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT", Splits: []SplitDetail{{UserID: "Alice", Value: 3000}}}
		if err := service.UpdateExpense(context.Background(), cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if auditSaves != 1 {
			t.Errorf("expected 1 audit save, got %d", auditSaves)
		}
	})

	t.Run("group swap writes deletion audit to old group then update to new group", func(t *testing.T) {
		var auditActions []domain.AuditAction
		eRepo := &mocks.MockExpenseRepo{
			GetByIDFunc: func(_ context.Context, _ domain.ExpenseID) (*domain.Expense, error) {
				return makeOldExpense("group-a"), nil
			},
		}
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(_ context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Members: []domain.UserID{"Alice"}}, nil
			},
		}
		aRepo := &mocks.MockAuditRepo{
			SaveFunc: func(_ context.Context, log domain.AuditLog) error {
				auditActions = append(auditActions, log.Action)
				return nil
			},
		}
		service := newTestExpenseService(eRepo, gRepo, aRepo)
		cmd := UpdateExpenseCommand{ID: "exp-1", GroupID: "group-b", Description: "Updated", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT", Splits: []SplitDetail{{UserID: "Alice", Value: 3000}}}
		if err := service.UpdateExpense(context.Background(), cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(auditActions) != 2 {
			t.Fatalf("expected 2 audit saves (delete+update), got %d", len(auditActions))
		}
		if auditActions[0] != domain.AuditActionDeletedExpense {
			t.Errorf("first audit should be DeletedExpense, got %s", auditActions[0])
		}
		if auditActions[1] != domain.AuditActionUpdatedExpense {
			t.Errorf("second audit should be UpdatedExpense, got %s", auditActions[1])
		}
	})

	t.Run("moving expense out of group writes only deletion audit for old group", func(t *testing.T) {
		var auditActions []domain.AuditAction
		eRepo := &mocks.MockExpenseRepo{
			GetByIDFunc: func(_ context.Context, _ domain.ExpenseID) (*domain.Expense, error) {
				return makeOldExpense("group-a"), nil
			},
		}
		aRepo := &mocks.MockAuditRepo{
			SaveFunc: func(_ context.Context, log domain.AuditLog) error {
				auditActions = append(auditActions, log.Action)
				return nil
			},
		}
		service := newTestExpenseService(eRepo, &mocks.MockGroupRepo{}, aRepo)
		cmd := UpdateExpenseCommand{ID: "exp-1", GroupID: "", Description: "Updated", TotalCents: 3000, Payer: "Alice", SplitType: "EXACT", Splits: []SplitDetail{{UserID: "Alice", Value: 3000}}}
		if err := service.UpdateExpense(context.Background(), cmd); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(auditActions) != 1 {
			t.Fatalf("expected 1 audit save (deletion only), got %d", len(auditActions))
		}
		if auditActions[0] != domain.AuditActionDeletedExpense {
			t.Errorf("expected DeletedExpense audit, got %s", auditActions[0])
		}
	})
}

func TestExpenseService_SettleUp(t *testing.T) {
	aRepo := &mocks.MockAuditRepo{}
	eRepo := &mocks.MockExpenseRepo{
		SaveFunc: func(ctx context.Context, expense *domain.Expense) error { return nil },
	}
	service := newTestExpenseService(eRepo, &mocks.MockGroupRepo{}, aRepo)

	t.Run("Fails if payer equals receiver", func(t *testing.T) {
		cmd := SettleUpCommand{PayerID: "Alice", ReceiverID: "Alice", AmountCents: 2000}
		if err := service.SettleUp(context.Background(), cmd); !errors.Is(err, domain.ErrSamePayerReceiver) {
			t.Errorf("expected ErrSamePayerReceiver, got %v", err)
		}
	})

	t.Run("Fails if amount is zero", func(t *testing.T) {
		cmd := SettleUpCommand{PayerID: "Alice", ReceiverID: "Bob", AmountCents: 0}
		if err := service.SettleUp(context.Background(), cmd); !errors.Is(err, domain.ErrInvalidSettlementAmount) {
			t.Errorf("expected ErrInvalidSettlementAmount, got %v", err)
		}
	})

	t.Run("Succeeds", func(t *testing.T) {
		cmd := SettleUpCommand{PayerID: "Alice", ReceiverID: "Bob", AmountCents: 1500}
		if err := service.SettleUp(context.Background(), cmd); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
