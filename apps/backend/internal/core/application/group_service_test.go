package application

import (
	"context"
	"errors"
	"testing"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/mocks"
	"ardoise/libs/shared/money"
)

func newTestGroupService(gRepo *mocks.MockGroupRepo, eRepo *mocks.MockExpenseRepo, aRepo *mocks.MockAuditRepo) *GroupService {
	return NewGroupService(gRepo, eRepo, aRepo, &mocks.MockInvitationRepo{}, &mocks.MockUserRepo{}, &mocks.MockTransactor{})
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

func TestGroupService_InviteUser_Errors(t *testing.T) {
	t.Run("actor not in group returns ErrUnauthorized", func(t *testing.T) {
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Name: "Trip", Members: []domain.UserID{"Alice"}}, nil
			},
		}
		service := newTestGroupService(gRepo, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{})
		err := service.InviteUserToGroup(context.Background(), "g1", "Bob", "Outsider")
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("invitee already a member returns ErrUserAlreadyInGroup", func(t *testing.T) {
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Name: "Trip", Members: []domain.UserID{"alice", "bob"}}, nil
			},
		}
		service := newTestGroupService(gRepo, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{})
		// "Bob" is normalized to "bob", which already belongs to the group.
		err := service.InviteUserToGroup(context.Background(), "g1", "Bob", "alice")
		if !errors.Is(err, domain.ErrUserAlreadyInGroup) {
			t.Errorf("expected ErrUserAlreadyInGroup, got %v", err)
		}
	})
}

func TestGroupService_InviteUser_SavesInvitation(t *testing.T) {
	invSaved := false
	invRepo := &mocks.MockInvitationRepo{
		SaveFunc: func(ctx context.Context, inv domain.Invitation) error {
			invSaved = true
			if inv.InviterID != "alice" || inv.InviteeID != "bob" {
				t.Errorf("unexpected invitation: inviter=%s invitee=%s", inv.InviterID, inv.InviteeID)
			}
			return nil
		},
	}
	gRepo := &mocks.MockGroupRepo{
		GetByIDFunc: func(ctx context.Context, id domain.GroupID) (*domain.Group, error) {
			return &domain.Group{ID: id, Name: "Trip", Members: []domain.UserID{"alice"}}, nil
		},
	}
	uRepo := &mocks.MockUserRepo{
		GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
			return &domain.User{ID: id, IsActive: true}, nil
		},
	}
	service := NewGroupService(gRepo, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, invRepo, uRepo, &mocks.MockTransactor{})

	if err := service.InviteUserToGroup(context.Background(), "g1", "Bob", "alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !invSaved {
		t.Error("expected invitationRepo.Save to be called for InviteUserToGroup")
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
					"exp-1", nil, "Dinner", total, "usera",
					[]domain.Split{{User: "usera", Amount: split}, {User: "userb", Amount: split}},
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
				exp1, _ := domain.NewExpense("exp-1", nil, "Lunch", total, "alice",
					[]domain.Split{{User: "alice", Amount: split}, {User: "bob", Amount: split}})
				exp2, _ := domain.NewExpense("exp-2", nil, "Dinner", total, "charlie",
					[]domain.Split{{User: "charlie", Amount: split}, {User: "alice", Amount: split}})
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

func TestGroupService_AcceptInvitation(t *testing.T) {
	baseInv := domain.Invitation{
		ID:        "inv-1",
		GroupID:   "g1",
		InviterID: "Alice",
		InviteeID: "Bob",
	}

	t.Run("adds invitee to group, deletes invitation, saves audit log", func(t *testing.T) {
		groupSaved := false
		invDeleted := false
		auditSaved := false

		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, id string) (domain.Invitation, error) {
				return baseInv, nil
			},
			DeleteFunc: func(_ context.Context, id string) error {
				invDeleted = true
				if id != "inv-1" {
					t.Errorf("expected inv-1, got %s", id)
				}
				return nil
			},
		}
		gRepo := &mocks.MockGroupRepo{
			GetByIDFunc: func(_ context.Context, id domain.GroupID) (*domain.Group, error) {
				return &domain.Group{ID: id, Name: "Trip", Members: []domain.UserID{"Alice"}}, nil
			},
			SaveFunc: func(_ context.Context, g *domain.Group) error {
				groupSaved = true
				if !g.HasMember("Bob") {
					t.Error("expected Bob to be added to group")
				}
				return nil
			},
		}
		aRepo := &mocks.MockAuditRepo{
			SaveFunc: func(_ context.Context, log domain.AuditLog) error {
				auditSaved = true
				if log.Action != domain.AuditActionAcceptedInvite {
					t.Errorf("expected action %s, got %s", domain.AuditActionAcceptedInvite, log.Action)
				}
				return nil
			},
		}
		service := NewGroupService(gRepo, &mocks.MockExpenseRepo{}, aRepo, invRepo, &mocks.MockUserRepo{}, &mocks.MockTransactor{})

		if err := service.AcceptInvitation(context.Background(), "inv-1", "Bob"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !groupSaved {
			t.Error("expected group to be saved")
		}
		if !invDeleted {
			t.Error("expected invitation to be deleted")
		}
		if !auditSaved {
			t.Error("expected audit log to be saved")
		}
	})

	t.Run("non-invitee actor returns ErrUnauthorized", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, _ string) (domain.Invitation, error) {
				return baseInv, nil
			},
		}
		service := NewGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, invRepo, &mocks.MockUserRepo{}, &mocks.MockTransactor{})

		err := service.AcceptInvitation(context.Background(), "inv-1", "Mallory")
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("invitation not found returns ErrInvitationNotFound", func(t *testing.T) {
		service := NewGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, &mocks.MockInvitationRepo{}, &mocks.MockUserRepo{}, &mocks.MockTransactor{})

		err := service.AcceptInvitation(context.Background(), "missing", "Bob")
		if !errors.Is(err, domain.ErrInvitationNotFound) {
			t.Errorf("expected ErrInvitationNotFound, got %v", err)
		}
	})
}

func TestGroupService_DeclineInvitation(t *testing.T) {
	baseInv := domain.Invitation{
		ID:        "inv-1",
		GroupID:   "g1",
		InviterID: "Alice",
		InviteeID: "Bob",
	}

	t.Run("deletes invitation", func(t *testing.T) {
		invDeleted := false
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, _ string) (domain.Invitation, error) {
				return baseInv, nil
			},
			DeleteFunc: func(_ context.Context, id string) error {
				invDeleted = true
				if id != "inv-1" {
					t.Errorf("expected inv-1, got %s", id)
				}
				return nil
			},
		}
		service := NewGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, invRepo, &mocks.MockUserRepo{}, &mocks.MockTransactor{})

		if err := service.DeclineInvitation(context.Background(), "inv-1", "Bob"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !invDeleted {
			t.Error("expected invitation to be deleted")
		}
	})

	t.Run("non-invitee actor returns ErrUnauthorized", func(t *testing.T) {
		invRepo := &mocks.MockInvitationRepo{
			GetByIDFunc: func(_ context.Context, _ string) (domain.Invitation, error) {
				return baseInv, nil
			},
		}
		service := NewGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, invRepo, &mocks.MockUserRepo{}, &mocks.MockTransactor{})

		err := service.DeclineInvitation(context.Background(), "inv-1", "Mallory")
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("invitation not found returns ErrInvitationNotFound", func(t *testing.T) {
		service := NewGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, &mocks.MockInvitationRepo{}, &mocks.MockUserRepo{}, &mocks.MockTransactor{})

		err := service.DeclineInvitation(context.Background(), "missing", "Bob")
		if !errors.Is(err, domain.ErrInvitationNotFound) {
			t.Errorf("expected ErrInvitationNotFound, got %v", err)
		}
	})
}

func TestGroupService_GetMyInvitations(t *testing.T) {
	t.Run("returns pending invitations for user", func(t *testing.T) {
		expected := []domain.Invitation{
			{ID: "inv-1", GroupID: "g1", InviterID: "Alice", InviteeID: "Bob"},
			{ID: "inv-2", GroupID: "g2", InviterID: "Charlie", InviteeID: "Bob"},
		}
		invRepo := &mocks.MockInvitationRepo{
			GetPendingForUserFunc: func(_ context.Context, userID domain.UserID) ([]domain.Invitation, error) {
				if userID != "Bob" {
					t.Errorf("expected userID Bob, got %s", userID)
				}
				return expected, nil
			},
		}
		service := NewGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, invRepo, &mocks.MockUserRepo{}, &mocks.MockTransactor{})

		got, err := service.GetMyInvitations(context.Background(), "Bob")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 invitations, got %d", len(got))
		}
	})

	t.Run("returns empty slice when no invitations", func(t *testing.T) {
		service := NewGroupService(&mocks.MockGroupRepo{}, &mocks.MockExpenseRepo{}, &mocks.MockAuditRepo{}, &mocks.MockInvitationRepo{}, &mocks.MockUserRepo{}, &mocks.MockTransactor{})

		got, err := service.GetMyInvitations(context.Background(), "Alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected 0 invitations, got %d", len(got))
		}
	})
}
