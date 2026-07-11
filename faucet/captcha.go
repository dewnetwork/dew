package faucet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CaptchaVerifier checks a client-side captcha token.
type CaptchaVerifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

// HTTPCaptchaVerifier talks to Cloudflare Turnstile or hCaptcha siteverify.
type HTTPCaptchaVerifier struct {
	Provider   CaptchaProvider
	Secret     string
	HTTPClient *http.Client
	// Endpoint overrides the default siteverify URL (tests).
	Endpoint string
}

func (v *HTTPCaptchaVerifier) endpoint() string {
	if v.Endpoint != "" {
		return v.Endpoint
	}
	switch v.Provider {
	case CaptchaTurnstile:
		return "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	case CaptchaHCaptcha:
		return "https://hcaptcha.com/siteverify"
	default:
		return ""
	}
}

// Verify posts the token to the captcha provider.
func (v *HTTPCaptchaVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("captcha token required")
	}
	ep := v.endpoint()
	if ep == "" {
		return fmt.Errorf("unknown captcha provider")
	}
	client := v.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	form := url.Values{}
	form.Set("secret", v.Secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("captcha verify request: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	var out struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("captcha verify decode: %w", err)
	}
	if !out.Success {
		if len(out.ErrorCodes) > 0 {
			return fmt.Errorf("captcha failed: %s", strings.Join(out.ErrorCodes, ", "))
		}
		return fmt.Errorf("captcha failed")
	}
	return nil
}

// StaticCaptcha accepts a fixed token (tests / offline).
type StaticCaptcha struct {
	Accept string
}

func (s StaticCaptcha) Verify(_ context.Context, token, _ string) error {
	if token != s.Accept {
		return fmt.Errorf("captcha failed")
	}
	return nil
}
