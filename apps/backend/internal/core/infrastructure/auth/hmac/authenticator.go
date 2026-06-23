package hmac

import (
	"context"

	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/domain"
	sharedjwt "ardoise/libs/shared/jwt"
)

// Authenticator verifies HS256 JWTs signed with a shared secret.
type Authenticator struct {
	secret []byte
}

var _ application.Authenticator = (*Authenticator)(nil)

func New(secret []byte) *Authenticator {
	return &Authenticator{secret: secret}
}

func (a *Authenticator) Authenticate(_ context.Context, token string) (domain.UserID, error) {
	sub, err := sharedjwt.Verify(token, a.secret)
	if err != nil {
		return "", domain.ErrUnauthorized
	}
	// Normalize so already-issued tokens carrying a mixed-case subject resolve to
	// the same canonical (lower-case) identity used everywhere else.
	return domain.UserID(domain.NormalizeUsername(sub)), nil
}
