package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"opensplit/apps/backend/internal/core/domain"
	"opensplit/apps/backend/internal/core/mocks"

	"golang.org/x/crypto/bcrypt"
)

func TestUserService_RegisterUser_DuplicateUsername(t *testing.T) {
	repo := &mocks.MockUserRepo{
		SaveFunc: func(ctx context.Context, user domain.User) error {
			return domain.ErrUserAlreadyExists
		},
	}
	service := NewUserService(repo, []byte("test-secret"))

	err := service.RegisterUser(context.Background(), "alice", "Alice", "password")
	if !errors.Is(err, domain.ErrUserAlreadyExists) {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestUserService_Auth(t *testing.T) {
	secret := []byte("test-secret")

	t.Run("RegisterUser hashes password securely", func(t *testing.T) {
		var savedHash string
		repo := &mocks.MockUserRepo{
			SaveFunc: func(ctx context.Context, user domain.User) error {
				savedHash = user.PasswordHash
				return nil
			},
		}
		service := NewUserService(repo, secret)

		err := service.RegisterUser(context.Background(), "Alice", "Alice S.", "my-password")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if savedHash == "" || savedHash == "my-password" {
			t.Error("expected password to be hashed, but it was stored as plain text or empty")
		}

		// verify it's a valid bcrypt hash
		if err := bcrypt.CompareHashAndPassword([]byte(savedHash), []byte("my-password")); err != nil {
			t.Error("stored hash does not match input password")
		}
	})

	t.Run("LoginUser returns valid JWT", func(t *testing.T) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
		repo := &mocks.MockUserRepo{
			GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
				return &domain.User{
					ID:           "Alice",
					IsActive:     true,
					PasswordHash: string(hash),
				}, nil
			},
		}
		service := NewUserService(repo, secret)

		_, err := service.LoginUser(context.Background(), "Alice", "wrong-password")
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}

		token, err := service.LoginUser(context.Background(), "Alice", "correct-password")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(strings.Split(token, ".")) != 3 {
			t.Error("expected a valid 3-part JWT string")
		}
	})
}

func TestUserService_ListUsers(t *testing.T) {
	secret := []byte("test-secret")
	repo := &mocks.MockUserRepo{
		ListAllFunc: func(ctx context.Context) ([]domain.User, error) {
			return []domain.User{{ID: "Alice", DisplayName: "Alice"}}, nil
		},
	}
	service := NewUserService(repo, secret)

	users, err := service.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(users) != 1 || users[0].ID != "Alice" {
		t.Errorf("unexpected user list returned")
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	repo := &mocks.MockUserRepo{
		UpdateFunc: func(ctx context.Context, id domain.UserID, newName string) error {
			return nil
		},
	}
	service := NewUserService(repo, []byte("test-secret"))

	t.Run("Fails with empty name", func(t *testing.T) {
		err := service.UpdateUser(context.Background(), "Alice", "")
		if !errors.Is(err, domain.ErrEmptyDisplayName) {
			t.Errorf("expected ErrEmptyDisplayName, got %v", err)
		}
	})

	t.Run("Succeeds with valid name", func(t *testing.T) {
		err := service.UpdateUser(context.Background(), "Alice", "Alice S.")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestUserService_DeleteUser(t *testing.T) {
	repo := &mocks.MockUserRepo{
		SoftDeleteFunc: func(ctx context.Context, id domain.UserID) error {
			return nil
		},
	}
	service := NewUserService(repo, []byte("test-secret"))

	err := service.DeleteUser(context.Background(), "Alice")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
