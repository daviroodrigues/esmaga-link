package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrTurnstileFailed = errors.New("verificação anti-bot (Turnstile) falhou ou expirou")

const defaultTurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type TurnstileVerifier struct {
	secretKey string
	verifyURL string
	client    *http.Client
}

func NewTurnstileVerifier(secretKey string) *TurnstileVerifier {
	return &TurnstileVerifier{
		secretKey: strings.TrimSpace(secretKey),
		verifyURL: defaultTurnstileVerifyURL,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (v *TurnstileVerifier) Enabled() bool {
	return v != nil && v.secretKey != ""
}

func (v *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if !v.Enabled() {
		return nil
	}
	if strings.TrimSpace(token) == "" {
		return ErrTurnstileFailed
	}

	form := url.Values{}
	form.Set("secret", v.secretKey)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("erro ao criar requisição Turnstile: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro ao validar Turnstile: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("erro ao decodificar resposta Turnstile: %w", err)
	}

	if !payload.Success {
		return ErrTurnstileFailed
	}
	return nil
}
