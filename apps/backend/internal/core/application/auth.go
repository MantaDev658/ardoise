package application

import (
	"context"

	"ardoise/apps/backend/internal/core/domain"
)

// Authenticator verifies a bearer token and returns the caller's identity.
// Implementations: HMAC (self-hosted JWT) and Clerk (RS256 via JWKS).
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (domain.UserID, error)
}
