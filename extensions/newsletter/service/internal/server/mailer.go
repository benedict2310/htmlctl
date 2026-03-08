package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"
)

const resendAPIURL = "https://api.resend.com/emails"

type NoopMailer struct{}

func (NoopMailer) SendVerification(context.Context, string, string) error {
	return nil
}

type LoggingMailer struct{}

func (LoggingMailer) SendVerification(context.Context, string, string) error {
	return fmt.Errorf("verification mailer not configured")
}

type ResendMailer struct {
	APIKey string
	From   string
	Client *http.Client
}

func NewResendMailer(apiKey, from string) ResendMailer {
	return ResendMailer{
		APIKey: strings.TrimSpace(apiKey),
		From:   strings.TrimSpace(from),
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (m ResendMailer) SendVerification(ctx context.Context, email, verifyURL string) error {
	htmlBody, err := renderVerificationEmailHTML(verifyURL)
	if err != nil {
		return fmt.Errorf("render verification email: %w", err)
	}

	_, err = m.SendCampaign(
		ctx,
		email,
		"Confirm your subscription",
		htmlBody,
		fmt.Sprintf("Confirm your subscription: %s\n\nIf you did not request this, you can ignore this email.", verifyURL),
	)
	return err
}

func (m ResendMailer) SendCampaign(ctx context.Context, email, subject, htmlBody, textBody string) (string, error) {
	if m.APIKey == "" {
		return "", fmt.Errorf("resend api key not configured")
	}
	if m.From == "" {
		return "", fmt.Errorf("resend from address not configured")
	}

	payload := map[string]any{
		"from":    m.From,
		"to":      []string{email},
		"subject": subject,
		"html":    htmlBody,
		"text":    textBody,
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return "", fmt.Errorf("encode resend payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, &body)
	if err != nil {
		return "", fmt.Errorf("create resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send resend request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resend returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed struct {
		ID string `json:"id"`
	}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &parsed)
	}
	return strings.TrimSpace(parsed.ID), nil
}

func renderVerificationEmailHTML(verifyURL string) (string, error) {
	const tpl = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Confirm your subscription</title>
</head>
<body style="margin:0;background:#0b0e14;color:#e0e5e9;font-family:Inter,-apple-system,Segoe UI,Roboto,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:24px 12px;background:#0b0e14;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#101722;border:1px solid rgba(255,255,255,0.12);border-radius:16px;overflow:hidden;">
          <tr>
            <td style="padding:28px 28px 10px;">
              <p style="margin:0 0 8px;font-size:12px;letter-spacing:.12em;text-transform:uppercase;color:#9db9c3;">Newsletter</p>
              <h1 style="margin:0 0 14px;font-size:28px;line-height:1.2;color:#ffffff;letter-spacing:-.02em;">Confirm your newsletter subscription</h1>
              <p style="margin:0 0 18px;font-size:15px;line-height:1.6;color:#b6c0c8;">
                Click the button below to verify your email address and activate your subscription.
              </p>
              <table role="presentation" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="border-radius:999px;background:#e0e5e9;">
                    <a href="{{.VerifyURL}}" style="display:inline-block;padding:12px 18px;font-size:14px;font-weight:700;text-decoration:none;color:#0b0e14;">Confirm subscription</a>
                  </td>
                </tr>
              </table>
              <p style="margin:18px 0 0;font-size:13px;line-height:1.6;color:#90a0aa;">
                If the button does not work, copy and paste this URL into your browser:
              </p>
              <p style="margin:6px 0 0;word-break:break-all;font-size:12px;color:#86b2c0;">{{.VerifyURL}}</p>
            </td>
          </tr>
          <tr>
            <td style="padding:14px 28px 24px;border-top:1px solid rgba(255,255,255,0.08);">
              <p style="margin:0;font-size:12px;color:#72818b;">If you did not request this email, you can safely ignore it.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

	t, err := template.New("verify-email").Parse(tpl)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, struct {
		VerifyURL string
	}{VerifyURL: verifyURL}); err != nil {
		return "", err
	}
	return out.String(), nil
}
