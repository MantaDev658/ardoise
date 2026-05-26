package clerk_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"ardoise/apps/backend/internal/core/domain"
	clerkauth "ardoise/apps/backend/internal/core/infrastructure/auth/clerk"
)

func TestClerkAuthenticator(t *testing.T) {
	// Generate an RSA key pair for the test JWKS server.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	pub := &privateKey.PublicKey

	const kid = "test-key-1"

	// Serve a minimal JWKS endpoint using the public key.
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"kid": kid,
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer jwksServer.Close()

	auth := clerkauth.New(jwksServer.URL)

	sign := func(sub string, exp time.Time) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub": sub,
			"exp": exp.Unix(),
		})
		token.Header["kid"] = kid
		signed, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("sign token: %v", err)
		}
		return signed
	}

	t.Run("accepts valid RS256 token", func(t *testing.T) {
		tokenString := sign("user_clerk123", time.Now().Add(time.Hour))
		id, err := auth.Authenticate(context.Background(), tokenString)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != domain.UserID("user_clerk123") {
			t.Errorf("got %q, want %q", id, "user_clerk123")
		}
	})

	t.Run("rejects expired token", func(t *testing.T) {
		tokenString := sign("user_clerk123", time.Now().Add(-time.Hour))
		_, err := auth.Authenticate(context.Background(), tokenString)
		if err == nil {
			t.Fatal("expected error for expired token, got nil")
		}
	})

	t.Run("rejects token signed with wrong key", func(t *testing.T) {
		otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub": "user_attacker",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		token.Header["kid"] = kid
		tokenString, _ := token.SignedString(otherKey)

		_, err := auth.Authenticate(context.Background(), tokenString)
		if err == nil {
			t.Fatal("expected error for wrong key, got nil")
		}
	})

	t.Run("rejects HS256 token", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user_attacker",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString([]byte("hmac-secret"))
		_, err := auth.Authenticate(context.Background(), tokenString)
		if err == nil {
			t.Fatal("expected error for HMAC token against RS256 authenticator, got nil")
		}
	})
}
