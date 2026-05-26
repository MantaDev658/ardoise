package jwt_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	sharedjwt "ardoise/libs/shared/jwt"
)

func TestSign(t *testing.T) {
	token, err := sharedjwt.Sign("alice", []byte("secret"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parts := splitDots(token)
	if len(parts) != 3 {
		t.Errorf("expected 3-part JWT, got %d parts", len(parts))
	}
}

func TestVerify(t *testing.T) {
	secret := []byte("secret")

	t.Run("round-trip", func(t *testing.T) {
		token, _ := sharedjwt.Sign("alice", secret)
		sub, err := sharedjwt.Verify(token, secret)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sub != "alice" {
			t.Errorf("expected sub 'alice', got %q", sub)
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		token, _ := sharedjwt.Sign("alice", secret)
		if _, err := sharedjwt.Verify(token, []byte("other")); err == nil {
			t.Error("expected error for wrong secret")
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		if _, err := sharedjwt.Verify("not.a.token", secret); err == nil {
			t.Error("expected error for malformed token")
		}
	})

	t.Run("alg:none rejected", func(t *testing.T) {
		tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"sub": "alice",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		tokenString, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if _, err := sharedjwt.Verify(tokenString, secret); err == nil {
			t.Error("expected error for alg:none token")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "alice",
			"exp": time.Now().Add(-time.Hour).Unix(),
		})
		tokenString, _ := tok.SignedString(secret)
		if _, err := sharedjwt.Verify(tokenString, secret); err == nil {
			t.Error("expected error for expired token")
		}
	})
}

func splitDots(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
