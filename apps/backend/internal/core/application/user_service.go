package application

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"ardoise/apps/backend/internal/core/domain"
	sharedjwt "ardoise/libs/shared/jwt"
)

type UserService struct {
	repo      domain.UserRepository
	jwtSecret []byte
}

func NewUserService(repo domain.UserRepository, secret []byte) *UserService {
	return &UserService{repo: repo, jwtSecret: secret}
}

func (s *UserService) RegisterUser(ctx context.Context, id, displayName, plainPassword string) error {
	if len([]byte(plainPassword)) > 72 {
		return domain.ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := domain.User{
		ID:           domain.UserID(domain.NormalizeUsername(id)),
		DisplayName:  displayName,
		PasswordHash: string(hash),
	}
	return s.repo.Save(ctx, user)
}

func (s *UserService) LoginUser(ctx context.Context, id, plainPassword string) (string, error) {
	user, err := s.repo.GetByID(ctx, domain.UserID(domain.NormalizeUsername(id)))
	if err != nil || !user.IsActive {
		return "", domain.ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(plainPassword)); err != nil {
		return "", domain.ErrInvalidCredentials
	}

	return sharedjwt.Sign(string(user.ID), s.jwtSecret)
}

func (s *UserService) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.repo.ListAll(ctx)
}

func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetByID(ctx, domain.UserID(id))
}

func (s *UserService) UpdateUser(ctx context.Context, id string, displayName string) error {
	if displayName == "" {
		return domain.ErrEmptyDisplayName
	}
	return s.repo.Update(ctx, domain.UserID(id), displayName)
}

func (s *UserService) ChangePassword(ctx context.Context, id, currentPlain, newPlain string) error {
	if len(newPlain) < 8 {
		return domain.ErrPasswordTooShort
	}
	if len([]byte(newPlain)) > 72 {
		return domain.ErrPasswordTooLong
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

func (s *UserService) ListFriends(ctx context.Context, userID string) ([]domain.User, error) {
	return s.repo.ListCoMembers(ctx, domain.UserID(userID))
}

// DeleteUser soft-deletes the account, preserving audit history.
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, domain.UserID(id))
}
