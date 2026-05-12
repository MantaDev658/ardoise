package application

import (
	"context"
	"fmt"

	"opensplit/apps/backend/internal/core/domain"

	"github.com/google/uuid"
)

type GroupService struct {
	groupRepo   domain.GroupRepository
	expenseRepo domain.ExpenseRepository
	auditRepo   domain.AuditRepository
	transactor  domain.Transactor
}

func NewGroupService(groupRepo domain.GroupRepository, expenseRepo domain.ExpenseRepository, auditRepo domain.AuditRepository, tx domain.Transactor) *GroupService {
	return &GroupService{
		groupRepo:   groupRepo,
		expenseRepo: expenseRepo,
		auditRepo:   auditRepo,
		transactor:  tx,
	}
}

type CreateGroupCommand struct {
	Name      string `json:"name"`
	CreatorID string `json:"-"` // set by the handler from JWT; never read from client input
}

func (c CreateGroupCommand) Validate() error {
	if c.Name == "" {
		return domain.ErrEmptyGroupName
	}
	return nil
}

func (s *GroupService) CreateGroup(ctx context.Context, cmd CreateGroupCommand) (string, error) {
	id := domain.GroupID(uuid.NewString())
	group, err := domain.NewGroup(id, cmd.Name, domain.UserID(cmd.CreatorID))
	if err != nil {
		return "", err
	}

	err = s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if saveErr := s.groupRepo.Save(txCtx, group); saveErr != nil {
			return fmt.Errorf("failed to save group: %w", saveErr)
		}
		return s.auditRepo.Save(txCtx, domain.AuditLog{
			ID:      uuid.NewString(),
			GroupID: string(group.ID),
			UserID:  cmd.CreatorID,
			Action:  domain.AuditActionCreatedGroup,
			Details: "Created group: " + group.Name,
		})
	})
	if err != nil {
		return "", err
	}
	return string(id), nil
}

func (s *GroupService) ListGroupsForUser(ctx context.Context, userID string) ([]*domain.Group, error) {
	return s.groupRepo.ListForUser(ctx, domain.UserID(userID))
}

func (s *GroupService) AddMemberToGroup(ctx context.Context, groupID string, userID string, actorID string) error {
	gID := domain.GroupID(groupID)
	uID := domain.UserID(userID)

	group, err := s.groupRepo.GetByID(ctx, gID)
	if err != nil {
		return fmt.Errorf("failed to fetch group: %w", err)
	}

	if err := group.AddMember(uID); err != nil {
		return err
	}

	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.groupRepo.Save(txCtx, group); err != nil {
			return fmt.Errorf("failed to save group member: %w", err)
		}
		return s.auditRepo.Save(txCtx, domain.AuditLog{
			ID:       uuid.NewString(),
			GroupID:  groupID,
			UserID:   actorID,
			Action:   domain.AuditActionAddedMember,
			TargetID: userID,
		})
	})
}

func (s *GroupService) UpdateGroup(ctx context.Context, groupID string, name string, actorID string) error {
	if name == "" {
		return domain.ErrEmptyGroupName
	}

	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.groupRepo.UpdateName(txCtx, domain.GroupID(groupID), name); err != nil {
			return fmt.Errorf("failed to update group name: %w", err)
		}
		return s.auditRepo.Save(txCtx, domain.AuditLog{
			ID:      uuid.NewString(),
			GroupID: groupID,
			UserID:  actorID,
			Action:  domain.AuditActionRenamedGroup,
			Details: "Renamed to " + name,
		})
	})
}

func (s *GroupService) DeleteGroup(ctx context.Context, groupID string, userID string) error {
	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.groupRepo.Delete(txCtx, domain.GroupID(groupID)); err != nil {
			return fmt.Errorf("failed to delete group: %w", err)
		}
		return s.auditRepo.Save(txCtx, domain.AuditLog{
			ID:       uuid.NewString(),
			GroupID:  groupID,
			UserID:   userID,
			Action:   domain.AuditActionDeletedGroup,
			TargetID: groupID,
		})
	})
}

// RemoveMember removes userID from the group, returning ErrOutstandingBalance if they still owe or are owed money.
// The balance check and the deletion run inside a single transaction behind a row-level lock on the group,
// preventing a concurrent AddExpense from inserting a debt between the check and the removal.
func (s *GroupService) RemoveMember(ctx context.Context, groupID string, userID string, actorID string) error {
	gID := domain.GroupID(groupID)
	uID := domain.UserID(userID)

	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.groupRepo.LockGroup(txCtx, gID); err != nil {
			return err
		}

		expenses, err := s.expenseRepo.ListByGroup(txCtx, gID, domain.Page{})
		if err != nil {
			return fmt.Errorf("failed to fetch group expenses for validation: %w", err)
		}

		pairwise := domain.CalculatePairwiseBalance(expenses, uID)
		for _, balance := range pairwise {
			if balance != 0 {
				return fmt.Errorf("%w with one or more members", domain.ErrOutstandingBalance)
			}
		}

		if err := s.groupRepo.RemoveMember(txCtx, gID, uID); err != nil {
			return fmt.Errorf("failed to remove group member: %w", err)
		}
		return s.auditRepo.Save(txCtx, domain.AuditLog{
			ID:       uuid.NewString(),
			GroupID:  groupID,
			UserID:   actorID,
			Action:   domain.AuditActionRemovedMember,
			TargetID: userID,
		})
	})
}
