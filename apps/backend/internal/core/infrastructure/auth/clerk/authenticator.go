package clerk

import (
	"context"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/domain"
)

// Authenticator verifies Clerk-issued RS256 JWTs via JWKS.
type Authenticator struct {
	jwks keyfunc.Keyfunc
}

var _ application.Authenticator = (*Authenticator)(nil)

// New creates an Authenticator that fetches and caches keys from jwksURL.
// Key refresh is handled automatically by the keyfunc library.
func New(jwksURL string) *Authenticator {
	jwks, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		// keyfunc.NewDefault does not return an error in practice for a valid URL;
		// validation happens on first key fetch.
		panic(fmt.Sprintf("clerk: failed to initialize JWKS client: %v", err))
	}
	return &Authenticator{jwks: jwks}
}

func (a *Authenticator) Authenticate(_ context.Context, tokenString string) (domain.UserID, error) {
	token, err := jwt.Parse(tokenString, a.jwks.Keyfunc)
	if err != nil || !token.Valid {
		return "", domain.ErrUnauthorized
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", domain.ErrUnauthorized
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", domain.ErrUnauthorized
	}
	return domain.UserID(sub), nil
}
