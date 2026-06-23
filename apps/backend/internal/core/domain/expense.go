package domain

import (
	"strings"
	"time"

	"ardoise/libs/shared/money"
)

type ExpenseID string

type UserID string

// NormalizeUsername canonicalizes a username for storage and lookup. Usernames are
// case-insensitive, so they are stored and compared in lower case. Surrounding
// whitespace is trimmed since usernames are often entered by hand (e.g. invitations).
func NormalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Split is a single participant's share of an expense.
type Split struct {
	User   UserID
	Amount money.Money
}

type Expense struct {
	id          ExpenseID
	groupID     *GroupID
	description string
	total       money.Money
	payer       UserID
	splits      []Split
	createdAt   time.Time
}

func NewExpense(id ExpenseID, groupID *GroupID, desc string, total money.Money, payer UserID, splits []Split) (*Expense, error) {
	if payer == "" {
		return nil, ErrMissingPayer
	}
	if len(splits) == 0 {
		return nil, ErrNoSplits
	}

	var calculatedTotal money.Money = 0
	for _, split := range splits {
		calculatedTotal = calculatedTotal.Add(split.Amount)
	}

	if calculatedTotal != total {
		return nil, ErrSplitsDoNotEqualTotal
	}

	return &Expense{
		id:          id,
		groupID:     groupID,
		description: desc,
		total:       total,
		payer:       payer,
		splits:      splits,
		createdAt:   time.Now().UTC(),
	}, nil
}

// NewExpenseFromDB trusts that the data is already valid and sets createdAt from the stored value.
func NewExpenseFromDB(id ExpenseID, groupID *GroupID, desc string, total money.Money, payer UserID, splits []Split, createdAt time.Time) (*Expense, error) {
	e, err := NewExpense(id, groupID, desc, total, payer, splits)
	if err != nil {
		return nil, err
	}
	e.createdAt = createdAt
	return e, nil
}

func (e *Expense) ID() ExpenseID        { return e.id }
func (e *Expense) GroupID() *GroupID    { return e.groupID }
func (e *Expense) Description() string  { return e.description }
func (e *Expense) Total() money.Money   { return e.total }
func (e *Expense) Payer() UserID        { return e.payer }
func (e *Expense) Splits() []Split      { return e.splits }
func (e *Expense) CreatedAt() time.Time { return e.createdAt }

// CalculatePairwiseBalance returns, for each other user, the net amount they owe
// userID (positive) or userID owes them (negative). Unlike CalculateNetBalances,
// this is not an aggregate — it catches zero-sum cases where a user's overall net
// is 0 but they still have live debts with specific individuals.
func CalculatePairwiseBalance(expenses []*Expense, userID UserID) map[UserID]int64 {
	net := make(map[UserID]int64)
	for _, exp := range expenses {
		if exp.Payer() == userID {
			for _, split := range exp.Splits() {
				if split.User != userID {
					net[split.User] += split.Amount.Int64()
				}
			}
		} else {
			for _, split := range exp.Splits() {
				if split.User == userID {
					net[exp.Payer()] -= split.Amount.Int64()
				}
			}
		}
	}
	return net
}

// CalculateNetBalances returns each user's net position across all expenses.
// A positive value means the user is owed money; negative means they owe money.
func CalculateNetBalances(expenses []*Expense) map[UserID]int64 {
	balances := make(map[UserID]int64)

	for _, exp := range expenses {
		balances[exp.Payer()] += exp.Total().Int64()
		for _, split := range exp.Splits() {
			balances[split.User] -= split.Amount.Int64()
		}
	}

	return balances
}
