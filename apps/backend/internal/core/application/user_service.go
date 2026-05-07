package application

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"opensplit/apps/backend/internal/core/domain"
)

// UserService implements user registration, authentication, and account management.
type UserService struct {
	repo      domain.UserRepository
	jwtSecret []byte
}

// NewUserService wires the repository and JWT signing secret into a UserService.
func NewUserService(repo domain.UserRepository, secret []byte) *UserService {
	return &UserService{repo: repo, jwtSecret: secret}
}

// CreateUserCommand carries the input for creating a basic user account.
type CreateUserCommand struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// CreateUser persists a basic user account without password authentication.
func (s *UserService) CreateUser(ctx context.Context, cmd CreateUserCommand) error {
	if cmd.ID == "" || cmd.DisplayName == "" {
		return fmt.Errorf("user ID and display name are required")
	}

	user := domain.User{
		ID:          domain.UserID(cmd.ID),
		DisplayName: cmd.DisplayName,
	}

	return s.repo.Save(ctx, user)
}

// RegisterUser hashes the password and persists the account.
func (s *UserService) RegisterUser(ctx context.Context, id, displayName, plainPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := domain.User{
		ID:           domain.UserID(id),
		DisplayName:  displayName,
		PasswordHash: string(hash),
	}
	return s.repo.Save(ctx, user)
}

// LoginUser verifies credentials and returns a signed JWT on success.
func (s *UserService) LoginUser(ctx context.Context, id, plainPassword string) (string, error) {
	user, err := s.repo.GetByID(ctx, domain.UserID(id))
	if err != nil || !user.IsActive {
		return "", domain.ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(plainPassword)); err != nil {
		return "", domain.ErrInvalidCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})
	return token.SignedString(s.jwtSecret)
}

// ListUsers returns all registered users.
func (s *UserService) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.repo.ListAll(ctx)
}

// UpdateUser changes the display name for the given user.
func (s *UserService) UpdateUser(ctx context.Context, id string, displayName string) error {
	if displayName == "" {
		return domain.ErrEmptyDisplayName
	}
	return s.repo.Update(ctx, domain.UserID(id), displayName)
}

// ChangePassword verifies the current password then replaces it with a newly hashed one.
// New password must be at least 8 characters and differ from the current one.
func (s *UserService) ChangePassword(ctx context.Context, id, currentPlain, newPlain string) error {
	if len(newPlain) < 8 {
		return domain.ErrPasswordTooShort
	}

	user, err := s.repo.GetByID(ctx, domain.UserID(id))
	if err != nil || !user.IsActive {
		return domain.ErrUserNotFound
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPlain)) != nil {
		return domain.ErrInvalidCredentials
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPlain)) == nil {
		return domain.ErrSamePassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPlain), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return s.repo.UpdatePassword(ctx, domain.UserID(id), string(newHash))
}

// DeleteUser soft-deletes the account, preserving audit history.
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, domain.UserID(id))
}
