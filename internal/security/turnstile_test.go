package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTurnstileVerifier_DisabledByDefault(t *testing.T) {
	v := NewTurnstileVerifier("")
	if v.Enabled() {
		t.Fatal("esperava Turnstile desabilitado quando secret está vazio")
	}
	if err := v.Verify(context.Background(), "", "127.0.0.1"); err != nil {
		t.Fatalf("esperava nil quando desabilitado, obteve %v", err)
	}
}

func TestTurnstileVerifier_Verify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		ok := r.FormValue("response") == "valid-token" && r.FormValue("secret") == "sec-123"
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": ok})
	}))
	defer srv.Close()

	v := NewTurnstileVerifier("sec-123")
	v.verifyURL = srv.URL

	if err := v.Verify(context.Background(), "valid-token", "127.0.0.1"); err != nil {
		t.Fatalf("esperava sucesso para token válido, obteve %v", err)
	}
	if err := v.Verify(context.Background(), "bad-token", "127.0.0.1"); err == nil {
		t.Fatal("esperava erro para token inválido")
	}
	if err := v.Verify(context.Background(), "", "127.0.0.1"); err == nil {
		t.Fatal("esperava erro para token vazio")
	}
}
