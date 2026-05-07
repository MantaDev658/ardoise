package domain

import "errors"

// --- Allocation Errors ---
var (
	ErrParticipantNotFound       = errors.New("must have at least one participant")
	ErrInvalidPercentages        = errors.New("percentages must sum to exactly 100.00")
	ErrInvalidNumberOfShares     = errors.New("total shares must be greater than zero")
	ErrInvalidAllocationStrategy = errors.New("unknown allocation strategy")
)

// --- Expense & Settlement Errors ---
var (
	ErrExpenseNotFound         = errors.New("expense not found")
	ErrInvalidTotal            = errors.New("invalid total amount")
	ErrSplitsDoNotEqualTotal   = errors.New("splits do not add up to the total amount")
	ErrSamePayerReceiver       = errors.New("payer and receiver cannot be the same person")
	ErrInvalidSettlementAmount = errors.New("settlement amount must be greater than zero")
	ErrMissingPayer            = errors.New("an expense must have a valid payer")
	ErrNoSplits                = errors.New("an expense must have at least one split")
)

// --- Group Errors ---
var (
	ErrGroupNotFound      = errors.New("group not found")
	ErrUserNotInGroup     = errors.New("user is not a member of the group")
	ErrUserAlreadyInGroup = errors.New("user is already a member of the group")
	ErrEmptyGroupName     = errors.New("group name cannot be empty")
	ErrOutstandingBalance = errors.New("cannot remove user with an outstanding balance")
)

// --- User Errors ---
var (
	ErrUserNotFound       = errors.New("user not found or inactive")
	ErrUserAlreadyExists  = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmptyDisplayName   = errors.New("display name cannot be empty")
	ErrUnauthorized       = errors.New("unauthorized: missing or invalid identity")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrSamePassword       = errors.New("new password must differ from the current password")
)
