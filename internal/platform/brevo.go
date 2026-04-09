package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const brevoSendURL = "https://api.brevo.com/v3/smtp/email"

type BrevoClient struct {
	apiKey      string
	senderEmail string
	senderName  string
	httpClient  *http.Client
}

func NewBrevoClient(apiKey, senderEmail, senderName string) *BrevoClient {
	return &BrevoClient{
		apiKey:      apiKey,
		senderEmail: senderEmail,
		senderName:  senderName,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// SendInvitationEmail sends a home invitation email via Brevo.
// inviteURL should be {FRONTEND_URL}/invitations/{token}.
func (c *BrevoClient) SendInvitationEmail(ctx context.Context, toEmail, toName, inviterName, homeName, inviteURL string) error {
	if toEmail == "" {
		return fmt.Errorf("brevo: recipient email is required")
	}

	type contact struct {
		Email string `json:"email"`
		Name  string `json:"name,omitempty"`
	}
	body := struct {
		Sender      contact   `json:"sender"`
		To          []contact `json:"to"`
		Subject     string    `json:"subject"`
		HTMLContent string    `json:"htmlContent"`
	}{
		Sender: contact{Email: c.senderEmail, Name: c.senderName},
		To:     []contact{{Email: toEmail, Name: toName}},
		Subject: fmt.Sprintf("You're invited to join %q on Sol", homeName),
		HTMLContent: buildInviteHTML(inviterName, homeName, inviteURL),
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("brevo: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, brevoSendURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("brevo: build request: %w", err)
	}
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("brevo: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("brevo: unexpected status %d: %s (%s)", resp.StatusCode, errBody.Message, errBody.Code)
	}

	return nil
}

func buildInviteHTML(inviterName, homeName, inviteURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family:sans-serif;color:#1a1a1a;max-width:560px;margin:0 auto;padding:24px">
  <h2 style="margin-bottom:8px">You've been invited!</h2>
  <p><strong>%s</strong> has invited you to join the home <strong>%s</strong> on Sol.</p>
  <p>Click the button below to accept or decline the invitation. This link expires in 7 days.</p>
  <a href="%s" style="display:inline-block;margin:16px 0;padding:12px 24px;background:#2563eb;color:#fff;text-decoration:none;border-radius:6px;font-weight:600">
    View Invitation
  </a>
  <p style="color:#6b7280;font-size:13px">If you weren't expecting this, you can safely ignore this email.</p>
</body>
</html>`, inviterName, homeName, inviteURL)
}
