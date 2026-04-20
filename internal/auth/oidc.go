package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/zitadel/zitadel-go/v3/pkg/authorization"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization/oauth"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
)

type OIDCVerifier struct {
	authZ *authorization.Authorizer[*oauth.IntrospectionContext]
}

func NewOIDCVerifier(ctx context.Context, issuer, keyFilePath string) (*OIDCVerifier, error) {
	domain := strings.TrimPrefix(strings.TrimPrefix(issuer, "https://"), "http://")

	authZ, err := authorization.New(ctx, zitadel.New(domain), oauth.DefaultAuthorization(keyFilePath))
	if err != nil {
		return nil, fmt.Errorf("zitadel auth init: %w", err)
	}
	return &OIDCVerifier{authZ: authZ}, nil
}

type Claims struct {
	Subject string
	Email   string
	Name    string
}

func (v *OIDCVerifier) Verify(ctx context.Context, token string) (*Claims, error) {
	introspection, err := v.authZ.CheckAuthorization(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("token verification: %w", err)
	}

	name := introspection.Name
	if name == "" {
		name = introspection.PreferredUsername
	}

	return &Claims{
		Subject: introspection.UserID(),
		Email:   introspection.Email,
		Name:    name,
	}, nil
}
