package application

import (
	"context"
	"fmt"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/libs/shared/money"

	"github.com/google/uuid"
)

type ExpenseService struct {
	expenseRepo domain.ExpenseRepository
	groupRepo   domain.GroupRepository
	transactor  domain.Transactor
}

func NewExpenseService(eRepo domain.ExpenseRepository, gRepo domain.GroupRepository, tx domain.Transactor) *ExpenseService {
	return &ExpenseService{
		expenseRepo: eRepo,
		groupRepo:   gRepo,
		transactor:  tx,
	}
}

type SplitDetail struct {
	UserID string  `json:"user_id"`
	Value  float64 `json:"value"`
}

type CreateExpenseCommand struct {
	GroupID     string        `json:"group_id,omitempty"`
	Description string        `json:"description"`
	TotalCents  int64         `json:"total_cents"`
	Payer       string        `json:"payer"`
	SplitType   string        `json:"split_type"`
	Splits      []SplitDetail `json:"splits"`
}

func (c CreateExpenseCommand) Validate() error {
	if c.TotalCents <= 0 {
		return domain.ErrInvalidTotal
	}
	return nil
}

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

func (s *ExpenseService) AddExpense(ctx context.Context, cmd CreateExpenseCommand) error {
	expense, err := s.buildAndValidateExpense(ctx, uuid.NewString(), cmd.GroupID, cmd.Description, cmd.TotalCents, cmd.Payer, cmd.SplitType, cmd.Splits)
	if err != nil {
		return fmt.Errorf("business rule violation: %w", err)
	}

	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if cmd.GroupID != "" {
			if err := s.groupRepo.LockGroup(txCtx, domain.GroupID(cmd.GroupID)); err != nil {
				return err
			}
		}

		if err := s.expenseRepo.Save(txCtx, expense); err != nil {
			return fmt.Errorf("infrastructure failure: %w", err)
		}
		return nil
	})
}

func (s *ExpenseService) ListExpensesForUser(ctx context.Context, userID string, page domain.Page) ([]*domain.Expense, error) {
	expenses, err := s.expenseRepo.ListForUser(ctx, domain.UserID(userID), page)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user expenses: %w", err)
	}
	return expenses, nil
}

func (s *ExpenseService) ListAllExpenses(ctx context.Context, page domain.Page) ([]*domain.Expense, error) {
	expenses, err := s.expenseRepo.ListAll(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch expenses: %w", err)
	}
	return expenses, nil
}

func (s *ExpenseService) ListExpensesByGroup(ctx context.Context, groupID string, page domain.Page) ([]*domain.Expense, error) {
	expenses, err := s.expenseRepo.ListByGroup(ctx, domain.GroupID(groupID), page)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch group expenses: %w", err)
	}
	return expenses, nil
}

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
		return nil
	})
}

func (s *ExpenseService) DeleteExpense(ctx context.Context, id string, userID string) error {
	return s.transactor.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.expenseRepo.Delete(txCtx, domain.ExpenseID(id)); err != nil {
			return fmt.Errorf("failed to delete expense: %w", err)
		}
		return nil
	})
}

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
	})
}
