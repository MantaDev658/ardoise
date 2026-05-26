package postgres

import (
	"context"
	"errors"
	"testing"

	"ardoise/apps/backend/internal/core/domain"
)

func TestUserRepository_DuplicateUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	user := domain.User{ID: "duplicate-user", DisplayName: "First"}
	if err := repo.Save(ctx, user); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	err := repo.Save(ctx, domain.User{ID: "duplicate-user", DisplayName: "Second"})
	if !errors.Is(err, domain.ErrUserAlreadyExists) {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	if err := repo.Save(ctx, domain.User{ID: "pw-user", DisplayName: "PW User", PasswordHash: "oldhash"}); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	if err := repo.UpdatePassword(ctx, "pw-user", "newhash"); err != nil {
		t.Fatalf("failed to update password: %v", err)
	}

	user, err := repo.GetByID(ctx, "pw-user")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if user.PasswordHash != "newhash" {
		t.Errorf("expected 'newhash', got %q", user.PasswordHash)
	}
}

func TestUserRepository_Lifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	user := domain.User{ID: "Charlie", DisplayName: "Charlie Brown"}
	err := repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	err = repo.Update(ctx, "Charlie", "Charles Brown")
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	err = repo.SoftDelete(ctx, "Charlie")
	if err != nil {
		t.Fatalf("failed to soft delete user: %v", err)
	}

	users, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}

	for _, u := range users {
		if u.ID == "Charlie" {
			t.Errorf("expected Charlie to be hidden by soft delete, but he was returned")
		}
	}
}
