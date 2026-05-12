package application

import (
	"context"
	"errors"
	"testing"

	"opensplit/apps/backend/internal/core/domain"
	"opensplit/apps/backend/internal/core/mocks"
	"opensplit/libs/shared/money"
)

func newTestGroupService(gRepo *mocks.MockGroupRepo, eRepo *mocks.MockExpenseRepo, aRepo *mocks.MockAuditRepo) *GroupService {
	return NewGroupService(gRepo, eRepo, aRepo, &mocks.MockTransactor{})
}

func TestGroupService_CRUD(t *testing.T) {
	gRepo := &mocks.MockGroupRepo{}
	eRepo := &mocks.MockExpenseRepo{}
	aRepo := &mocks.MockAuditRepo{}
	service := newTestGroupService(gRepo, eRepo, aRepo)

	t.Run("UpdateGroup fails on empty name", func(t *testing.T) {
		err := service.UpdateGroup(context.Background(), "g1", "", "u1")
		if !errors.Is(err, domain.ErrEmptyGroupName) {
			t.Errorf("expected ErrEmptyGroupName, got %v", err)
		}
	})

	t.Run("DeleteGroup succeeds", func(t *testing.T) {
		err := service.DeleteGroup(context.Background(), "g1", "u1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestGroupService_CreateGroup_SavesAuditLog(t *testing.T) {
	auditSaved := false
	aRepo := &mocks.MockAuditRepo{
		SaveFunc: func(ctx context.Context, log domain.AuditLog) error {
			auditSaved = true
			if log.Action != domain.AuditActionCreatedGroup {
				t.Errorf("expected action %s, got %s", domain.AuditActionCreatedGroup, log.Action)
			}
			return nil
		},
	}
	gRepo := &mocks.MockGroupRepo{}
	service := newTestGroupService(gRepo, &mocks.MockExpenseRepo{}, aRepo)

	_, err := service.CreateGroup(context.Background(), CreateGroupCommand{Name: "Trip", CreatorID: "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !auditSaved {
		t.Error("expected auditRepo.Save to be called for CreateGroup")
	}
}

func TestGroupService_UpdateGroup_SavesAuditLog(t *testing.T) {
	auditSaved := false
	aRepo := &mocks.MockAuditRepo{
		SaveFunc: func(ctx context.Context, log domain.AuditLog) error {
			auditSaved = true
			if log.Action != domain.AuditActionRenamedGroup {
				t.Errorf("expected action %s, got %s", domain.AuditActionRenamedGroup, log.Action)
			}
			return nil
		},
	}
	service := newTestGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, aRepo)

	if err := service.UpdateGroup(context.Background(), "g1", "New Name", "Alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !auditSaved {
		t.Error("expected auditRepo.Save to be called for UpdateGroup")
	}
}

func TestGroupService_DeleteGroup_SavesAuditLog(t *testing.T) {
	auditSaved := false
	aRepo := &mocks.MockAuditRepo{
		SaveFunc: func(ctx context.Context, log domain.AuditLog) error {
			auditSaved = true
			if log.Action != domain.AuditActionDeletedGroup {
				t.Errorf("expected action %s, got %s", domain.AuditActionDeletedGroup, log.Action)
			}
			return nil
		},
	}
	service := newTestGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, aRepo)

	if err := service.DeleteGroup(context.Background(), "g1", "Alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !auditSaved {
		t.Error("expected auditRepo.Save to be called for DeleteGroup")
	}
}

func TestGroupService_AddMember_SavesAuditLog(t *testing.T) {
	auditSaved := false
	aRepo := &mocks.MockAuditRepo{
		SaveFunc: func(ctx context.Context, log domain.AuditLog) error {
			auditSaved = true
			if log.Action != domain.AuditActionAddedMember {
				t.Errorf("expected action %s, got %s", domain.AuditActionAddedMember, log.Action)
			}
			return nil
		},
	}
	gRepo := &mocks.MockGroupRepo{
		GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
			return &domain.Group{ID: id, Name: "Trip", Members: []domain.UserID{"Alice"}}, nil
		},
	}
	service := newTestGroupService(gRepo, &mocks.MockExpenseRepo{}, aRepo)

	if err := service.AddMemberToGroup(context.Background(), "g1", "Bob", "Alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !auditSaved {
		t.Error("expected auditRepo.Save to be called for AddMemberToGroup")
	}
}

func TestGroupService_RemoveMember_SavesAuditLog(t *testing.T) {
	auditSaved := false
	aRepo := &mocks.MockAuditRepo{
		SaveFunc: func(ctx context.Context, log domain.AuditLog) error {
			auditSaved = true
			if log.Action != domain.AuditActionRemovedMember {
				t.Errorf("expected action %s, got %s", domain.AuditActionRemovedMember, log.Action)
			}
			return nil
		},
	}
	service := newTestGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, aRepo)

	if err := service.RemoveMember(context.Background(), "g1", "Bob", "Alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !auditSaved {
		t.Error("expected auditRepo.Save to be called for RemoveMember")
	}
}

func TestGroupService_RemoveMember_AcquiresGroupLock(t *testing.T) {
	lockCalled := false
	gRepo := &mocks.MockGroupRepo{
		LockGroupFunc: func(_ context.Context, id domain.GroupID) error {
			lockCalled = true
			return nil
		},
	}
	service := newTestGroupService(gRepo, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{})

	if err := service.RemoveMember(context.Background(), "g1", "Bob", "Alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !lockCalled {
		t.Error("expected LockGroup to be called before balance check and deletion")
	}
}

func TestGroupService_RemoveMember_BalanceValidation(t *testing.T) {
	aRepo := &mocks.MockAuditRepo{}
	gRepo := &mocks.MockGroupRepo{}

	t.Run("Fails if user has an outstanding balance", func(t *testing.T) {
		eRepo := &mocks.MockExpenseRepo{
			ListByGroupFunc: func(ctx context.Context, groupID domain.GroupID, page domain.Page) ([]*domain.Expense, error) {
				total, _ := money.New(3000)
				split, _ := money.New(1500)
				exp, _ := domain.NewExpense(
					"exp-1", nil, "Dinner", total, "UserA",
					[]domain.Split{{User: "UserA", Amount: split}, {User: "UserB", Amount: split}},
				)
				return []*domain.Expense{exp}, nil
			},
		}

		service := newTestGroupService(gRepo, eRepo, aRepo)

		err := service.RemoveMember(context.Background(), "g1", "UserB", "a1")
		if !errors.Is(err, domain.ErrOutstandingBalance) {
			t.Errorf("expected ErrOutstandingBalance, got %v", err)
		}
	})

	t.Run("Succeeds if user balance is exactly zero", func(t *testing.T) {
		eRepo := &mocks.MockExpenseRepo{}
		service := newTestGroupService(gRepo, eRepo, aRepo)

		err := service.RemoveMember(context.Background(), "g1", "UserC", "a1")
		if err != nil {
			t.Errorf("expected success, got: %v", err)
		}
	})

	t.Run("Fails when pairwise debts exist despite zero aggregate net (zero-sum trap)", func(t *testing.T) {
		// Alice pays $50 split with Bob → Bob owes Alice $25
		// Charlie pays $50 split with Alice → Alice owes Charlie $25
		// Alice's aggregate net is 0, but she has live bilateral debts on both sides.
		eRepo := &mocks.MockExpenseRepo{
			ListByGroupFunc: func(_ context.Context, _ domain.GroupID, _ domain.Page) ([]*domain.Expense, error) {
				total, _ := money.New(5000)
				split, _ := money.New(2500)
				exp1, _ := domain.NewExpense("exp-1", nil, "Lunch", total, "Alice",
					[]domain.Split{{User: "Alice", Amount: split}, {User: "Bob", Amount: split}})
				exp2, _ := domain.NewExpense("exp-2", nil, "Dinner", total, "Charlie",
					[]domain.Split{{User: "Charlie", Amount: split}, {User: "Alice", Amount: split}})
				return []*domain.Expense{exp1, exp2}, nil
			},
		}
		service := newTestGroupService(gRepo, eRepo, aRepo)

		err := service.RemoveMember(context.Background(), "g1", "Alice", "admin")
		if !errors.Is(err, domain.ErrOutstandingBalance) {
			t.Errorf("expected ErrOutstandingBalance (zero-sum trap), got %v", err)
		}
	})
}
