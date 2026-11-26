package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTurnstileVerifier_DisabledByDefault(t *testing.T) {
	v := NewTurnstileVerifier("", "")
	if v.Enabled() {
		t.Fatal("esperava Turnstile desabilitado quando secret está vazio")
	}
	if err := v.Verify(context.Background(), "", "127.0.0.1", "create_link"); err != nil {
		t.Fatalf("esperava nil quando desabilitado, obteve %v", err)
	}
}

func TestTurnstileVerifier_Verify(t *testing.T) {
	var lastRemoteIP string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		lastRemoteIP = r.FormValue("remoteip")
		switch r.FormValue("response") {
		case "valid-token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":  r.FormValue("secret") == "sec-123",
				"action":   "create_link",
				"hostname": "localhost",
			})
		case "wrong-action-token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":  true,
				"action":   "register",
				"hostname": "localhost",
			})
		case "wrong-host-token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":  true,
				"action":   "create_link",
				"hostname": "evil.example.com",
			})
		case "upstream-error":
			w.WriteHeader(http.StatusBadGateway)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false})
		}
	}))
	defer srv.Close()

	v := NewTurnstileVerifier("sec-123", "localhost, 127.0.0.1")
	v.verifyURL = srv.URL

	if err := v.Verify(context.Background(), "valid-token", "127.0.0.1:54321", "create_link"); err != nil {
		t.Fatalf("esperava sucesso para token válido, obteve %v", err)
	}
	if lastRemoteIP != "127.0.0.1" {
		t.Fatalf("esperava remoteip sem porta (127.0.0.1), obteve %q", lastRemoteIP)
	}
	if err := v.Verify(context.Background(), "wrong-action-token", "127.0.0.1", "create_link"); err == nil {
		t.Fatal("esperava erro para action divergente")
	}
	if err := v.Verify(context.Background(), "wrong-host-token", "127.0.0.1", "create_link"); err == nil {
		t.Fatal("esperava erro para hostname não permitido")
	}
	if err := v.Verify(context.Background(), "upstream-error", "127.0.0.1", "create_link"); err == nil {
		t.Fatal("esperava erro para resposta HTTP não-2xx")
	}
	if err := v.Verify(context.Background(), "bad-token", "127.0.0.1", "create_link"); err == nil {
		t.Fatal("esperava erro para token inválido")
	}
	if err := v.Verify(context.Background(), "", "127.0.0.1", "create_link"); err == nil {
		t.Fatal("esperava erro para token vazio")
	}
	if err := v.Verify(context.Background(), strings.Repeat("a", maxTurnstileTokenLength+1), "127.0.0.1", "create_link"); err == nil {
		t.Fatal("esperava erro para token acima de 2048 caracteres")
	}

	noHosts := NewTurnstileVerifier("sec-123", "")
	noHosts.verifyURL = srv.URL
	if err := noHosts.Verify(context.Background(), "valid-token", "127.0.0.1", "create_link"); err == nil {
		t.Fatal("esperava erro quando TURNSTILE_HOSTNAMES está vazio")
	}

	urlHosts := NewTurnstileVerifier("sec-123", "https://localhost:8080/, https://esmaga.adomtech.com.br")
	urlHosts.verifyURL = srv.URL
	if err := urlHosts.Verify(context.Background(), "valid-token", "127.0.0.1", "create_link"); err != nil {
		t.Fatalf("esperava sucesso ao normalizar URL em TURNSTILE_HOSTNAMES, obteve %v", err)
	}
}
