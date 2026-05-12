package domain

import (
	"context"
	"time"
)

// Transactor wraps multiple repository operations in a single atomic database transaction.
type Transactor interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Page controls pagination for list queries. Limit=0 means no limit.
// Cursor is the created_at of the last item seen; zero means start from the beginning.
// CursorID is the id of the last item seen, used as a tie-breaker when two rows
// share the same created_at timestamp (prevents rows from being silently skipped).
type Page struct {
	Limit    int
	Cursor   time.Time
	CursorID string
}

// FriendBalance is the aggregated net balance between two users across all non-group expenses.
// Positive NetCents means the friend owes the user; negative means the user owes the friend.
type FriendBalance struct {
	FriendID UserID
	NetCents int64
}

// AuditAction is the event type recorded in an audit log entry.
type AuditAction string

const (
	AuditActionCreatedExpense AuditAction = "CREATED_EXPENSE"
	AuditActionUpdatedExpense AuditAction = "UPDATED_EXPENSE"
	AuditActionDeletedExpense AuditAction = "DELETED_EXPENSE"
	AuditActionSettledDebt    AuditAction = "SETTLED_DEBT"
	AuditActionCreatedGroup   AuditAction = "CREATED_GROUP"
	AuditActionAddedMember    AuditAction = "ADDED_MEMBER"
	AuditActionRenamedGroup   AuditAction = "RENAMED_GROUP"
	AuditActionDeletedGroup   AuditAction = "DELETED_GROUP"
	AuditActionRemovedMember  AuditAction = "REMOVED_GROUP_MEMBER"
)

// AuditLog is an immutable record of a mutation within a group.
type AuditLog struct {
	ID        string      `json:"id"`
	GroupID   string      `json:"group_id"`
	UserID    string      `json:"user_id"`
	Action    AuditAction `json:"action"`
	TargetID  string      `json:"target_id,omitempty"`
	Details   string      `json:"details,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

// User is a registered account.
type User struct {
	ID           UserID
	DisplayName  string
	IsActive     bool
	PasswordHash string
}

type AuditRepository interface {
	Save(ctx context.Context, log AuditLog) error
	ListByGroup(ctx context.Context, groupID GroupID, page Page) ([]AuditLog, error)
}

type UserRepository interface {
	Save(ctx context.Context, user User) error
	GetByID(ctx context.Context, id UserID) (*User, error)
	ListAll(ctx context.Context) ([]User, error)
	ListCoMembers(ctx context.Context, userID UserID) ([]User, error)
	Update(ctx context.Context, userID UserID, displayName string) error
	UpdatePassword(ctx context.Context, userID UserID, newHash string) error
	SoftDelete(ctx context.Context, userId UserID) error
}

type GroupRepository interface {
	Save(ctx context.Context, group *Group) error
	GetByID(ctx context.Context, id GroupID) (*Group, error)
	ListForUser(ctx context.Context, userID UserID) ([]*Group, error)
	UpdateName(ctx context.Context, id GroupID, name string) error
	Delete(ctx context.Context, id GroupID) error
	RemoveMember(ctx context.Context, id GroupID, userID UserID) error
	LockGroup(ctx context.Context, id GroupID) error
}

type ExpenseRepository interface {
	Save(ctx context.Context, expense *Expense) error
	GetByID(ctx context.Context, id ExpenseID) (*Expense, error)
	ListAll(ctx context.Context, page Page) ([]*Expense, error)
	ListForUser(ctx context.Context, userID UserID, page Page) ([]*Expense, error)
	ListByGroup(ctx context.Context, groupID GroupID, page Page) ([]*Expense, error)
	GetFriendBalanceSummary(ctx context.Context, userID UserID) ([]FriendBalance, error)
	Update(ctx context.Context, expense *Expense) error
	Delete(ctx context.Context, id ExpenseID) error
}
