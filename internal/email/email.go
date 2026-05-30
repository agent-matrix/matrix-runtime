// Package email sends transactional mail (account verification, password
// resets) via Resend (https://resend.com). It is configured entirely from the
// environment — no secrets are ever hardcoded:
//
//	RESEND_API_KEY     Resend API key (re_...). When empty, the sender runs in
//	                   "log only" mode: messages are logged, not sent, so local
//	                   dev works without an account.
//	MATRIXCLOUD_EMAIL_FROM   From address, e.g. "MatrixCloud <noreply@matrixhub.io>".
//	MATRIXCLOUD_APP_URL      Public console URL used to build links (default
//	                         https://cloud.matrixhub.io).
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

// Sender delivers transactional email.
type Sender struct {
	apiKey string
	from   string
	appURL string
	http   *http.Client
}

// NewFromEnv builds a Sender from environment variables.
func NewFromEnv() *Sender {
	from := os.Getenv("MATRIXCLOUD_EMAIL_FROM")
	if from == "" {
		from = "MatrixCloud <onboarding@resend.dev>"
	}
	appURL := os.Getenv("MATRIXCLOUD_APP_URL")
	if appURL == "" {
		appURL = "https://cloud.matrixhub.io"
	}
	return &Sender{
		apiKey: os.Getenv("RESEND_API_KEY"),
		from:   from,
		appURL: strings.TrimRight(appURL, "/"),
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether real delivery is configured.
func (s *Sender) Enabled() bool { return s.apiKey != "" }

// AppURL returns the configured public console URL (no trailing slash).
func (s *Sender) AppURL() string { return s.appURL }

// Send delivers an HTML email. With no API key it logs and returns nil so that
// flows (signup, reset) still succeed in development.
func (s *Sender) Send(ctx context.Context, to, subject, html string) error {
	if s.apiKey == "" {
		log.Printf("email (log-only, no RESEND_API_KEY): to=%s subject=%q", to, subject)
		return nil
	}
	body, _ := json.Marshal(map[string]any{
		"from":    s.from,
		"to":      []string{to},
		"subject": subject,
		"html":    html,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("resend: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// SendWelcome sends a welcome + (optional) email-verification message.
func (s *Sender) SendWelcome(ctx context.Context, to, name, verifyToken string) error {
	greeting := name
	if greeting == "" {
		greeting = "there"
	}
	var verify string
	if verifyToken != "" {
		link := s.appURL + "/verify?token=" + verifyToken
		verify = fmt.Sprintf(`<p>Please confirm your email to activate your workspace:</p>
		<p><a href="%s" style="background:#4f46e5;color:#fff;padding:10px 18px;border-radius:8px;text-decoration:none;display:inline-block">Verify my email</a></p>
		<p style="color:#6b7280;font-size:13px">Or paste this link: %s</p>`, link, link)
	}
	html := fmt.Sprintf(`<div style="font-family:ui-sans-serif,system-ui,Segoe UI,Roboto,Arial;max-width:560px;margin:auto">
	<h2 style="margin:0 0 8px">Welcome to MatrixCloud, %s 👋</h2>
	<p>Your workspace is ready. Spin up sandboxes, run MatrixShell, inspect models, and plug in your own Hugging Face account to use HF LLMs inside your runtimes.</p>
	%s
	<p style="color:#6b7280;font-size:13px;margin-top:24px">If you didn't create this account, you can ignore this email.</p>
	</div>`, greeting, verify)
	return s.Send(ctx, to, "Welcome to MatrixCloud", html)
}

// SendPasswordReset sends a password-reset link.
func (s *Sender) SendPasswordReset(ctx context.Context, to, resetToken string) error {
	link := s.appURL + "/reset?token=" + resetToken
	html := fmt.Sprintf(`<div style="font-family:ui-sans-serif,system-ui,Segoe UI,Roboto,Arial;max-width:560px;margin:auto">
	<h2 style="margin:0 0 8px">Reset your MatrixCloud password</h2>
	<p>We received a request to reset your password. This link expires in 1 hour and can be used once.</p>
	<p><a href="%s" style="background:#4f46e5;color:#fff;padding:10px 18px;border-radius:8px;text-decoration:none;display:inline-block">Choose a new password</a></p>
	<p style="color:#6b7280;font-size:13px">Or paste this link: %s</p>
	<p style="color:#6b7280;font-size:13px;margin-top:24px">If you didn't request this, no action is needed — your password stays the same.</p>
	</div>`, link, link)
	return s.Send(ctx, to, "Reset your MatrixCloud password", html)
}
