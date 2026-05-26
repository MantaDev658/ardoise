package hmac_test

import (
	"context"
	"testing"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/infrastructure/auth/hmac"
	sharedjwt "ardoise/libs/shared/jwt"
)

func TestHMACAuthenticator(t *testing.T) {
	secret := []byte("test-secret")
	auth := hmac.New(secret)

	t.Run("returns UserID for valid token", func(t *testing.T) {
		token, err := sharedjwt.Sign("user-123", secret)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		id, err := auth.Authenticate(context.Background(), token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != domain.UserID("user-123") {
			t.Errorf("got %q, want %q", id, "user-123")
		}
	})

	t.Run("rejects token signed with wrong secret", func(t *testing.T) {
		token, _ := sharedjwt.Sign("user-123", []byte("wrong-secret"))
		_, err := auth.Authenticate(context.Background(), token)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("rejects malformed token", func(t *testing.T) {
		_, err := auth.Authenticate(context.Background(), "not.a.token")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
