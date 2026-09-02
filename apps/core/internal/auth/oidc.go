package auth

import (
	"context"
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

type userinfoResponse struct {
	Subject    string `json:"sub"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Username   string `json:"preferred_username"`
}

// Verify validates the access token with the issuer's UserInfo endpoint. This
// keeps authentication sound even when the service is reached without Traefik.
func (v *OIDCVerifier) Verify(ctx context.Context, token string) (*Claims, error) {
	issuer := strings.TrimRight(os.Getenv("OIDC_ISSUER"), "/")
	if issuer == "" {
		return nil, fmt.Errorf("OIDC_ISSUER is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/oidc/v1/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("verify token with userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("read userinfo response: %w", err)
	}
	var info userinfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse userinfo response: %w", err)
	}
	if info.Subject == "" {
		return nil, fmt.Errorf("userinfo response missing sub")
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
	return &Claims{Subject: info.Subject, Email: info.Email, Name: composedName}, nil
}

type Claims struct {
	Subject string
	Email   string
	Name    string
}
