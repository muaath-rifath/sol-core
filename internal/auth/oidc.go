package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type OIDCVerifier struct{}

func NewOIDCVerifier() *OIDCVerifier {
	return &OIDCVerifier{}
}

type Claims struct {
	Subject string
	Email   string
	Name    string
}

// Verify decodes the JWT payload without signature verification.
// Token integrity is guaranteed by Traefik's ForwardAuth middleware, which
// validates every request against Zitadel's userinfo endpoint before it
// reaches this service.
func (v *OIDCVerifier) Verify(_ context.Context, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}

	var raw struct {
		Sub      string `json:"sub"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Username string `json:"preferred_username"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse JWT claims: %w", err)
	}
	if raw.Sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	name := raw.Name
	if name == "" {
		name = raw.Username
	}

	return &Claims{
		Subject: raw.Sub,
		Email:   raw.Email,
		Name:    name,
	}, nil
}
