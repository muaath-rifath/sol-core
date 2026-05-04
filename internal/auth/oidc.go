package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type OIDCVerifier struct {
	httpClient *http.Client
}

func NewOIDCVerifier() *OIDCVerifier {
	return &OIDCVerifier{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Userinfo fetches name/email/preferred_username from the OIDC userinfo endpoint
// using the access token. Used when the JWT body lacks profile claims (Zitadel
// access tokens don't always include them). Returns empty strings on any error;
// callers should treat that as "no enrichment available" rather than fatal.
func (v *OIDCVerifier) Userinfo(ctx context.Context, token string) (email, name string) {
	issuer := strings.TrimRight(os.Getenv("OIDC_ISSUER"), "/")
	if issuer == "" {
		return "", ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/oidc/v1/userinfo", nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", ""
	}
	var info struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Username   string `json:"preferred_username"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", ""
	}
	composedName := info.Name
	if composedName == "" {
		parts := []string{}
		if info.GivenName != "" {
			parts = append(parts, info.GivenName)
		}
		if info.FamilyName != "" {
			parts = append(parts, info.FamilyName)
		}
		composedName = strings.TrimSpace(strings.Join(parts, " "))
		if composedName == "" {
			composedName = info.Username
		}
	}
	return info.Email, composedName
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
