package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrTurnstileFailed = errors.New("verificação anti-bot (Turnstile) falhou ou expirou")

const (
	defaultTurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	maxTurnstileTokenLength   = 2048
)

type TurnstileVerifier struct {
	secretKey        string
	allowedHostnames map[string]struct{}
	verifyURL        string
	client           *http.Client
}

func NewTurnstileVerifier(secretKey, hostnamesCSV string) *TurnstileVerifier {
	allowed := make(map[string]struct{})
	for _, raw := range strings.Split(hostnamesCSV, ",") {
		h := strings.ToLower(strings.TrimSpace(raw))
		if h != "" {
			allowed[h] = struct{}{}
		}
	}
	return &TurnstileVerifier{
		secretKey:        strings.TrimSpace(secretKey),
		allowedHostnames: allowed,
		verifyURL:        defaultTurnstileVerifyURL,
		client:           &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *TurnstileVerifier) Enabled() bool {
	return v != nil && v.secretKey != ""
}

func (v *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP, expectedAction string) error {
	if !v.Enabled() {
		return nil
	}
	token = strings.TrimSpace(token)
	expectedAction = strings.TrimSpace(expectedAction)
	if token == "" || len(token) > maxTurnstileTokenLength || expectedAction == "" || len(v.allowedHostnames) == 0 {
		return ErrTurnstileFailed
	}

	form := url.Values{}
	form.Set("secret", v.secretKey)
	form.Set("response", token)
	if ip := normalizeRemoteIP(remoteIP); ip != "" {
		form.Set("remoteip", ip)
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

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ErrTurnstileFailed
	}

	var payload struct {
		Success  bool   `json:"success"`
		Action   string `json:"action"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("erro ao decodificar resposta Turnstile: %w", err)
	}

	hostname := strings.ToLower(strings.TrimSpace(payload.Hostname))
	if _, ok := v.allowedHostnames[hostname]; !payload.Success || payload.Action != expectedAction || !ok {
		return ErrTurnstileFailed
	}
	return nil
}

func normalizeRemoteIP(remoteIP string) string {
	remoteIP = strings.TrimSpace(remoteIP)
	if remoteIP == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(remoteIP); err == nil {
		return host
	}
	return remoteIP
}
