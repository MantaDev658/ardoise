package application

import (
	"context"
	"fmt"

	"opensplit/apps/backend/internal/core/domain"
	"opensplit/libs/shared/money"

	"github.com/google/uuid"
)

// ExpenseService implements expense, balance, settlement, and activity use cases.
type ExpenseService struct {
	expenseRepo domain.ExpenseRepository
	groupRepo   domain.GroupRepository
	auditRepo   domain.AuditRepository
	transactor  domain.Transactor
}

// NewExpenseService wires the repositories and transactor into an ExpenseService.
func NewExpenseService(eRepo domain.ExpenseRepository, gRepo domain.GroupRepository, aRepo domain.AuditRepository, tx domain.Transactor) *ExpenseService {
	return &ExpenseService{
		expenseRepo: eRepo,
		groupRepo:   gRepo,
		auditRepo:   aRepo,
		transactor:  tx,
	}
}

// SplitDetail is the per-participant split value supplied by an API client.
type SplitDetail struct {
	UserID string  `json:"user_id"`
	Value  float64 `json:"value"`
}

// CreateExpenseCommand carries the input for creating a new expense.
type CreateExpenseCommand struct {
	GroupID             string             `json:"group_id,omitempty"`
	Description         string             `json:"description"`
	TotalCents          int64              `json:"total_cents"`
	Payer               string             `json:"payer"`
	SplitType           string             `json:"split_type"`
	Splits              []SplitDetail      `json:"splits"`
	AuditActionOverride domain.AuditAction `json:"-"`
}

func (c CreateExpenseCommand) Validate() error {
	if c.TotalCents <= 0 {
		return domain.ErrInvalidTotal
	}
	return nil
}

// UpdateExpenseCommand carries the input for replacing an existing expense.
type UpdateExpenseCommand struct {
	ID          string        `json:"id"`
	GroupID     string        `json:"group_id,omitempty"`
	Description string        `json:"description"`
	TotalCents  int64         `json:"total_cents"`
	Payer       string        `json:"payer"`
	SplitType   string        `json:"split_type"`
	Splits      []SplitDetail `json:"splits"`
}

func (c UpdateExpenseCommand) Validate() error {
	if c.TotalCents <= 0 {
		return domain.ErrInvalidTotal
	}
	return nil
}

// SettleUpCommand carries the input for recording a debt payment.
type SettleUpCommand struct {
	GroupID     string `json:"group_id,omitempty"`
	PayerID     string `json:"payer_id"`
	ReceiverID  string `json:"receiver_id"`
	AmountCents int64  `json:"amount_cents"`
}

func (s *ExpenseService) buildAndValidateExpense(ctx context.Context, id string, groupID string, desc string, totalCents int64, payer string, splitType string, inputSplits []SplitDetail) (*domain.Expense, error) {
	var domainInputs []domain.AllocationInput
	for _, split := range inputSplits {
		domainInputs = append(domainInputs, domain.AllocationInput{
			UserID: domain.UserID(split.UserID),
			Value:  split.Value,
		})
	}

	splits, err := domain.Allocate(domain.AllocationType(splitType), totalCents, domainInputs)
	if err != nil {
		return nil, fmt.Errorf("allocation math error: %w", err)
	}

	totalMoney, err := money.New(totalCents)
	if err != nil {
		return nil, domain.ErrInvalidTotal
	}

	var groupIDPtr *domain.GroupID
	if groupID != "" {
		gID := domain.GroupID(groupID)
		groupIDPtr = &gID

		group, groupErr := s.groupRepo.GetByID(ctx, gID)
		if groupErr != nil {
			return nil, fmt.Errorf("failed to validate group: %w", groupErr)
		}

		if !group.HasMember(domain.UserID(payer)) {
			return nil, fmt.Errorf("%w: payer %s is not a member of group %s", domain.ErrUserNotInGroup, payer, groupID)
		}

		for _, split := range splits {
			if !group.HasMember(split.User) {
				return nil, fmt.Errorf("split participant %s is not a member of group %s", split.User, group.Name)
			}
		}
	}

	return domain.NewExpense(domain.ExpenseID(id), groupIDPtr, desc, totalMoney, domain.UserID(payer), splits)
}

// AddExpense validates the command, allocates splits, and persists the expense.
func (s *ExpenseService) AddExpense(ctx context.Context, cmd CreateExpenseCommand) error {
	expense, err := s.buildAndValidateExpense(ctx, uuid.NewString(), cmd.GroupID, cmd.Description, cmd.TotalCents, cmd.Payer, cmd.SplitType, cmd.Splits)
	if err != nil {
		return fmt.Errorf("business rule violation: %w", err)
	}

	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.expenseRepo.Save(txCtx, expense); err != nil {
			return fmt.Errorf("infrastructure failure: %w", err)
		}

		if cmd.GroupID != "" {
			action := domain.AuditActionCreatedExpense
			if cmd.AuditActionOverride != "" {
				action = cmd.AuditActionOverride
			}
			return s.auditRepo.Save(txCtx, domain.AuditLog{
				ID:       uuid.NewString(),
				GroupID:  cmd.GroupID,
				UserID:   cmd.Payer,
				Action:   action,
				TargetID: string(expense.ID()),
				Details:  cmd.Description,
			})
		}
		return nil
	})
}

// ListExpensesForUser returns expenses where the given user is the payer or a split participant.
func (s *ExpenseService) ListExpensesForUser(ctx context.Context, userID string, page domain.Page) ([]*domain.Expense, error) {
	expenses, err := s.expenseRepo.ListForUser(ctx, domain.UserID(userID), page)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user expenses: %w", err)
	}
	return expenses, nil
}

// ListAllExpenses returns a paginated list of all expenses across all groups.
func (s *ExpenseService) ListAllExpenses(ctx context.Context, page domain.Page) ([]*domain.Expense, error) {
	expenses, err := s.expenseRepo.ListAll(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch expenses: %w", err)
	}
	return expenses, nil
}

// ListExpensesByGroup returns a paginated list of expenses scoped to a specific group.
func (s *ExpenseService) ListExpensesByGroup(ctx context.Context, groupID string, page domain.Page) ([]*domain.Expense, error) {
	expenses, err := s.expenseRepo.ListByGroup(ctx, domain.GroupID(groupID), page)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch group expenses: %w", err)
	}
	return expenses, nil
}

// GetFriendBalances returns the bilateral net balance between userID and each of their peers.
func (s *ExpenseService) GetFriendBalances(ctx context.Context, userID string) ([]domain.Transaction, error) {
	balances, err := s.expenseRepo.GetFriendBalanceSummary(ctx, domain.UserID(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch friend balances: %w", err)
	}

	result := make([]domain.Transaction, 0, len(balances))
	for _, b := range balances {
		amt := b.NetCents
		if amt < 0 {
			amt = -amt
		}
		if b.NetCents > 0 {
			result = append(result, domain.Transaction{From: b.FriendID, To: domain.UserID(userID), Amount: amt})
		} else {
			result = append(result, domain.Transaction{From: domain.UserID(userID), To: b.FriendID, Amount: amt})
		}
	}
	return result, nil
}

// GetGroupActivity returns a paginated audit log for a group.
func (s *ExpenseService) GetGroupActivity(ctx context.Context, groupID string, page domain.Page) ([]domain.AuditLog, error) {
	return s.auditRepo.ListByGroup(ctx, domain.GroupID(groupID), page)
}

// UpdateExpense validates the command and replaces the stored expense.
func (s *ExpenseService) UpdateExpense(ctx context.Context, cmd UpdateExpenseCommand) error {
	expense, err := s.buildAndValidateExpense(ctx, cmd.ID, cmd.GroupID, cmd.Description, cmd.TotalCents, cmd.Payer, cmd.SplitType, cmd.Splits)
	if err != nil {
		return fmt.Errorf("business rule violation: %w", err)
	}

	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.expenseRepo.Update(txCtx, expense); err != nil {
			return fmt.Errorf("infrastructure failure: %w", err)
		}

		if cmd.GroupID != "" {
			return s.auditRepo.Save(txCtx, domain.AuditLog{
				ID:       uuid.NewString(),
				GroupID:  cmd.GroupID,
				UserID:   cmd.Payer,
				Action:   domain.AuditActionUpdatedExpense,
				TargetID: string(expense.ID()),
				Details:  "Updated: " + cmd.Description,
			})
		}
		return nil
	})
}

// DeleteExpense removes an expense and records the deletion in the audit log.
func (s *ExpenseService) DeleteExpense(ctx context.Context, id string, userID string) error {
	expense, err := s.expenseRepo.GetByID(ctx, domain.ExpenseID(id))
	if err != nil {
		return err
	}

	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.expenseRepo.Delete(txCtx, domain.ExpenseID(id)); err != nil {
			return fmt.Errorf("failed to delete expense: %w", err)
		}

		if expense.GroupID() != nil {
			return s.auditRepo.Save(txCtx, domain.AuditLog{
				ID:       uuid.NewString(),
				GroupID:  string(*expense.GroupID()),
				UserID:   userID,
				Action:   domain.AuditActionDeletedExpense,
				TargetID: id,
				Details:  "Deleted expense: " + expense.Description(),
			})
		}
		return nil
	})
}

// SettleUp records a payment from payer to receiver as an exact-split expense.
func (s *ExpenseService) SettleUp(ctx context.Context, cmd SettleUpCommand) error {
	if cmd.PayerID == cmd.ReceiverID {
		return domain.ErrSamePayerReceiver
	}
	if cmd.AmountCents <= 0 {
		return domain.ErrInvalidSettlementAmount
	}

	return s.AddExpense(ctx, CreateExpenseCommand{
		GroupID:     cmd.GroupID,
		Description: "Payment",
		TotalCents:  cmd.AmountCents,
		Payer:       cmd.PayerID,
		SplitType:   string(domain.AllocationTypeExact),
		Splits: []SplitDetail{
			{UserID: cmd.ReceiverID, Value: float64(cmd.AmountCents)},
		},
		AuditActionOverride: domain.AuditActionSettledDebt,
	})
}
