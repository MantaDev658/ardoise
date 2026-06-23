package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/mocks"

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

	t.Run("RegisterUser rejects password over 72 bytes", func(t *testing.T) {
		repo := &mocks.MockUserRepo{}
		service := NewUserService(repo, secret)
		err := service.RegisterUser(context.Background(), "Alice", "Alice S.", strings.Repeat("a", 73))
		if !errors.Is(err, domain.ErrPasswordTooLong) {
			t.Errorf("expected ErrPasswordTooLong, got %v", err)
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

func TestUserService_ChangePassword(t *testing.T) {
	const currentPlain = "correct-current"
	hash, _ := bcrypt.GenerateFromPassword([]byte(currentPlain), bcrypt.DefaultCost)

	makeRepo := func() *mocks.MockUserRepo {
		return &mocks.MockUserRepo{
			GetByIDFunc: func(_ context.Context, _ domain.UserID) (*domain.User, error) {
				return &domain.User{ID: "Alice", IsActive: true, PasswordHash: string(hash)}, nil
			},
		}
	}

	service := NewUserService(makeRepo(), []byte("test-secret"))

	t.Run("rejects password shorter than 8 chars", func(t *testing.T) {
		err := service.ChangePassword(context.Background(), "Alice", currentPlain, "short")
		if !errors.Is(err, domain.ErrPasswordTooShort) {
			t.Errorf("expected ErrPasswordTooShort, got %v", err)
		}
	})

	t.Run("rejects new password over 72 bytes", func(t *testing.T) {
		err := service.ChangePassword(context.Background(), "Alice", currentPlain, strings.Repeat("a", 73))
		if !errors.Is(err, domain.ErrPasswordTooLong) {
			t.Errorf("expected ErrPasswordTooLong, got %v", err)
		}
	})

	t.Run("rejects wrong current password", func(t *testing.T) {
		err := service.ChangePassword(context.Background(), "Alice", "wrong-password", "newpassword123")
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("rejects same password as current", func(t *testing.T) {
		err := service.ChangePassword(context.Background(), "Alice", currentPlain, currentPlain)
		if !errors.Is(err, domain.ErrSamePassword) {
			t.Errorf("expected ErrSamePassword, got %v", err)
		}
	})

	t.Run("stores a new bcrypt hash on success", func(t *testing.T) {
		var storedHash string
		repo := makeRepo()
		repo.UpdatePasswordFunc = func(_ context.Context, _ domain.UserID, h string) error {
			storedHash = h
			return nil
		}
		svc := NewUserService(repo, []byte("test-secret"))

		err := svc.ChangePassword(context.Background(), "Alice", currentPlain, "newpassword123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if storedHash == "" || storedHash == "newpassword123" {
			t.Error("expected a hashed value, got plain text or empty")
		}
		if bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("newpassword123")) != nil {
			t.Error("stored hash does not match the new password")
		}
	})
}

func TestUserService_NormalizesUsername(t *testing.T) {
	secret := []byte("test-secret")

	t.Run("RegisterUser stores the username in lower case", func(t *testing.T) {
		var saved domain.UserID
		repo := &mocks.MockUserRepo{
			SaveFunc: func(ctx context.Context, user domain.User) error {
				saved = user.ID
				return nil
			},
		}
		service := NewUserService(repo, secret)

		if err := service.RegisterUser(context.Background(), "Alice", "Alice", "password"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if saved != "alice" {
			t.Errorf("expected stored id 'alice', got %q", saved)
		}
	})

	t.Run("LoginUser looks up the normalized username", func(t *testing.T) {
		var looked domain.UserID
		hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.DefaultCost)
		repo := &mocks.MockUserRepo{
			GetByIDFunc: func(ctx context.Context, id domain.UserID) (*domain.User, error) {
				looked = id
				return &domain.User{ID: id, IsActive: true, PasswordHash: string(hash)}, nil
			},
		}
		service := NewUserService(repo, secret)

		if _, err := service.LoginUser(context.Background(), "ALICE", "pw"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if looked != "alice" {
			t.Errorf("expected lookup by 'alice', got %q", looked)
		}
	})
}
