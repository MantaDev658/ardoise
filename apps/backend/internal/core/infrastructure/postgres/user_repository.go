package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"opensplit/apps/backend/internal/core/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Save(ctx context.Context, user domain.User) error {
	query := `
		INSERT INTO users (id, display_name, password_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING
	`
	res, err := r.db.ExecContext(ctx, query, string(user.ID), user.DisplayName, user.PasswordHash)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.ErrUserAlreadyExists
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	query := "SELECT id, display_name, password_hash, is_active FROM users WHERE id = $1"
	var u domain.User
	err := r.db.QueryRowContext(ctx, query, string(id)).Scan(&u.ID, &u.DisplayName, &u.PasswordHash, &u.IsActive)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) ListAll(ctx context.Context) ([]domain.User, error) {
	query := "SELECT id, display_name FROM users WHERE is_active = TRUE ORDER BY created_at ASC"
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.DisplayName); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, id domain.UserID, newName string) error {
	res, err := r.db.ExecContext(ctx, "UPDATE users SET display_name = $1 WHERE id = $2 AND is_active = TRUE", newName, string(id))
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) SoftDelete(ctx context.Context, id domain.UserID) error {
	res, err := r.db.ExecContext(ctx, "UPDATE users SET is_active = FALSE WHERE id = $1", string(id))
	if err != nil {
		return fmt.Errorf("failed to soft delete user: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}
