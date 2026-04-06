package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewOIDCVerifier(ctx context.Context, issuer, clientID string) (*OIDCVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
	return &OIDCVerifier{verifier: verifier}, nil
}

type Claims struct {
	Subject string
	Email   string
	Name    string
}

func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (*Claims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("token verification: %w", err)
	}

	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("claims extraction: %w", err)
	}
	claims.Subject = idToken.Subject

	return &claims, nil
}
