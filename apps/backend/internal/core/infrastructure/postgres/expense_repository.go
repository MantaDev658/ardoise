package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/libs/shared/money"
)

type ExpenseRepository struct {
	db *sql.DB
}

func NewExpenseRepository(db *sql.DB) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

func mapRowsToExpenses(rows *sql.Rows) ([]*domain.Expense, error) {
	type rawExpense struct {
		id          string
		groupID     sql.NullString
		description string
		totalCents  int64
		payer       string
		splits      []domain.Split
		createdAt   time.Time
	}
	expenseMap := make(map[string]*rawExpense)
	var orderedIDs []string

	for rows.Next() {
		var expID, desc, payer, splitUser string
		var exGroupID sql.NullString
		var totalCents, splitCents int64
		var createdAt time.Time

		if err := rows.Scan(&expID, &exGroupID, &desc, &totalCents, &payer, &createdAt, &splitUser, &splitCents); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if _, exists := expenseMap[expID]; !exists {
			expenseMap[expID] = &rawExpense{
				id:          expID,
				groupID:     exGroupID,
				description: desc,
				totalCents:  totalCents,
				payer:       payer,
				createdAt:   createdAt,
			}
			orderedIDs = append(orderedIDs, expID)
		}

		splitAmt, _ := money.New(splitCents)
		expenseMap[expID].splits = append(expenseMap[expID].splits, domain.Split{
			User:   domain.UserID(splitUser),
			Amount: splitAmt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	var results []*domain.Expense
	for _, id := range orderedIDs {
		raw := expenseMap[id]
		totalMoney, _ := money.New(raw.totalCents)
		var groupIDPtr *domain.GroupID
		if raw.groupID.Valid {
			gID := domain.GroupID(raw.groupID.String)
			groupIDPtr = &gID
		}
		exp, err := domain.NewExpenseFromDB(
			domain.ExpenseID(raw.id),
			groupIDPtr,
			raw.description,
			totalMoney,
			domain.UserID(raw.payer),
			raw.splits,
			raw.createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("corrupted data in db for expense %s: %w", id, err)
		}
		results = append(results, exp)
	}

	return results, nil
}

func insertExpense(ctx context.Context, tx *sql.Tx, expense *domain.Expense) error {
	var dbGroupID interface{}
	if expense.GroupID() != nil {
		dbGroupID = string(*expense.GroupID())
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO expenses (id, group_id, description, total_cents, payer_id)
		VALUES ($1, $2, $3, $4, $5)
	`, expense.ID(), dbGroupID, expense.Description(), expense.Total().Int64(), expense.Payer())
	if err != nil {
		return fmt.Errorf("failed to insert expense: %w", err)
	}

	for _, split := range expense.Splits() {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO splits (expense_id, user_id, amount_cents) VALUES ($1, $2, $3)`,
			expense.ID(), split.User, split.Amount.Int64(),
		)
		if err != nil {
			return fmt.Errorf("failed to insert split: %w", err)
		}
	}
	return nil
}

func (r *ExpenseRepository) Save(ctx context.Context, expense *domain.Expense) error {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return insertExpense(ctx, tx, expense)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertExpense(ctx, tx, expense); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ExpenseRepository) GetByID(ctx context.Context, id domain.ExpenseID) (*domain.Expense, error) {
	var desc, payer string
	var totalCents int64
	var dbGroupID sql.NullString
	err := r.db.QueryRowContext(ctx, "SELECT group_id, description, total_cents, payer_id FROM expenses WHERE id = $1", id).
		Scan(&dbGroupID, &desc, &totalCents, &payer)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrExpenseNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to query expense: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, "SELECT user_id, amount_cents FROM splits WHERE expense_id = $1", id)
	if err != nil {
		return nil, fmt.Errorf("failed to query splits: %w", err)
	}
	defer rows.Close()

	var splits []domain.Split
	for rows.Next() {
		var userID string
		var amountCents int64
		if err := rows.Scan(&userID, &amountCents); err != nil {
			return nil, fmt.Errorf("failed to scan split: %w", err)
		}
		amount, _ := money.New(amountCents)
		splits = append(splits, domain.Split{User: domain.UserID(userID), Amount: amount})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	var groupIDPtr *domain.GroupID
	if dbGroupID.Valid {
		gID := domain.GroupID(dbGroupID.String)
		groupIDPtr = &gID
	}

	totalMoney, _ := money.New(totalCents)
	return domain.NewExpense(id, groupIDPtr, desc, totalMoney, domain.UserID(payer), splits)
}

func (r *ExpenseRepository) ListAll(ctx context.Context, page domain.Page) ([]*domain.Expense, error) {
	// The LIMIT must apply to distinct expenses, not the expense×splits join, so
	// paginate expenses in a subquery and join splits for exactly those rows.
	var args []any
	inner := `
		SELECT id, group_id, description, total_cents, payer_id, created_at
		FROM expenses
	`
	if !page.Cursor.IsZero() {
		args = append(args, page.Cursor, page.CursorID)
		inner += fmt.Sprintf(" WHERE (created_at > $%d OR (created_at = $%d AND id > $%d))", len(args)-1, len(args)-1, len(args))
	}
	inner += " ORDER BY created_at ASC, id ASC"
	if page.Limit > 0 {
		args = append(args, page.Limit)
		inner += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	const outer = `
		SELECT e.id, e.group_id, e.description, e.total_cents, e.payer_id, e.created_at, s.user_id, s.amount_cents
		FROM (`
	query := outer + inner + `) e
		JOIN splits s ON e.id = s.expense_id
		ORDER BY e.created_at ASC, e.id ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list expenses: %w", err)
	}
	defer rows.Close()
	return mapRowsToExpenses(rows)
}

func (r *ExpenseRepository) ListForUser(ctx context.Context, userID domain.UserID, page domain.Page) ([]*domain.Expense, error) {
	// The LIMIT must apply to distinct expenses, not the expense×splits join, so
	// paginate expenses in a subquery and join splits for exactly those rows.
	args := []any{string(userID)}
	inner := `
		SELECT id, group_id, description, total_cents, payer_id, created_at
		FROM expenses
		WHERE (payer_id = $1 OR id IN (SELECT expense_id FROM splits WHERE user_id = $1))
	`
	if !page.Cursor.IsZero() {
		args = append(args, page.Cursor, page.CursorID)
		inner += fmt.Sprintf(" AND (created_at > $%d OR (created_at = $%d AND id > $%d))", len(args)-1, len(args)-1, len(args))
	}
	inner += " ORDER BY created_at ASC, id ASC"
	if page.Limit > 0 {
		args = append(args, page.Limit)
		inner += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	const outer = `
		SELECT e.id, e.group_id, e.description, e.total_cents, e.payer_id, e.created_at, s.user_id, s.amount_cents
		FROM (`
	query := outer + inner + `) e
		JOIN splits s ON e.id = s.expense_id
		ORDER BY e.created_at ASC, e.id ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list user expenses: %w", err)
	}
	defer rows.Close()
	return mapRowsToExpenses(rows)
}

func (r *ExpenseRepository) ListByGroup(ctx context.Context, groupID domain.GroupID, page domain.Page) ([]*domain.Expense, error) {
	// The LIMIT must apply to distinct expenses, not the expense×splits join, so
	// paginate expenses in a subquery and join splits for exactly those rows.
	args := []any{string(groupID)}
	inner := `
		SELECT id, group_id, description, total_cents, payer_id, created_at
		FROM expenses
		WHERE group_id = $1
	`
	if !page.Cursor.IsZero() {
		args = append(args, page.Cursor, page.CursorID)
		inner += fmt.Sprintf(" AND (created_at > $%d OR (created_at = $%d AND id > $%d))", len(args)-1, len(args)-1, len(args))
	}
	inner += " ORDER BY created_at ASC, id ASC"
	if page.Limit > 0 {
		args = append(args, page.Limit)
		inner += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	const outer = `
		SELECT e.id, e.group_id, e.description, e.total_cents, e.payer_id, e.created_at, s.user_id, s.amount_cents
		FROM (`
	query := outer + inner + `) e
		JOIN splits s ON e.id = s.expense_id
		ORDER BY e.created_at ASC, e.id ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list group expenses: %w", err)
	}
	defer rows.Close()
	return mapRowsToExpenses(rows)
}

// GetFriendBalanceSummary returns the net balance between userID and each of their friends
// across all non-group expenses. Positive NetCents means the friend owes the user.
func (r *ExpenseRepository) GetFriendBalanceSummary(ctx context.Context, userID domain.UserID) ([]domain.FriendBalance, error) {
	query := `
		SELECT
			CASE WHEN e.payer_id = $1 THEN s.user_id ELSE e.payer_id END AS friend_id,
			SUM(CASE WHEN e.payer_id = $1 THEN s.amount_cents ELSE -s.amount_cents END) AS net_cents
		FROM expenses e
		JOIN splits s ON e.id = s.expense_id
		WHERE e.group_id IS NULL
		  AND (
		    (e.payer_id = $1 AND s.user_id != $1)
		    OR (s.user_id = $1 AND e.payer_id != $1)
		  )
		GROUP BY friend_id
		HAVING SUM(CASE WHEN e.payer_id = $1 THEN s.amount_cents ELSE -s.amount_cents END) != 0
	`
	rows, err := r.db.QueryContext(ctx, query, string(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to query friend balances: %w", err)
	}
	defer rows.Close()

	var balances []domain.FriendBalance
	for rows.Next() {
		var b domain.FriendBalance
		var friendID string
		if err := rows.Scan(&friendID, &b.NetCents); err != nil {
			return nil, fmt.Errorf("failed to scan friend balance: %w", err)
		}
		b.FriendID = domain.UserID(friendID)
		balances = append(balances, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	return balances, nil
}

func updateExpense(ctx context.Context, tx *sql.Tx, expense *domain.Expense) error {
	var dbGroupID interface{}
	if expense.GroupID() != nil {
		dbGroupID = string(*expense.GroupID())
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE expenses
		SET group_id = $1, description = $2, total_cents = $3, payer_id = $4
		WHERE id = $5
	`, dbGroupID, expense.Description(), expense.Total().Int64(), expense.Payer(), string(expense.ID()))
	if err != nil {
		return fmt.Errorf("failed to update expense: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrExpenseNotFound
	}

	if _, delErr := tx.ExecContext(ctx, "DELETE FROM splits WHERE expense_id = $1", string(expense.ID())); delErr != nil {
		return fmt.Errorf("failed to delete old splits during update: %w", delErr)
	}

	for _, split := range expense.Splits() {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO splits (expense_id, user_id, amount_cents) VALUES ($1, $2, $3)`,
			string(expense.ID()), string(split.User), split.Amount.Int64(),
		)
		if err != nil {
			return fmt.Errorf("failed to insert new split: %w", err)
		}
	}
	return nil
}

// Update fully replaces an existing expense's details and rewrites its splits.
func (r *ExpenseRepository) Update(ctx context.Context, expense *domain.Expense) error {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return updateExpense(ctx, tx, expense)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateExpense(ctx, tx, expense); err != nil {
		return err
	}
	return tx.Commit()
}

func deleteExpense(ctx context.Context, tx *sql.Tx, id domain.ExpenseID) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM splits WHERE expense_id = $1", string(id)); err != nil {
		return fmt.Errorf("failed to delete splits: %w", err)
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM expenses WHERE id = $1", string(id))
	if err != nil {
		return fmt.Errorf("failed to delete expense: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrExpenseNotFound
	}
	return nil
}

// Delete completely removes an expense and its associated splits.
func (r *ExpenseRepository) Delete(ctx context.Context, id domain.ExpenseID) error {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return deleteExpense(ctx, tx, id)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := deleteExpense(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit()
}
