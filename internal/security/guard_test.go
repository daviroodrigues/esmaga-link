package security

import (
	"net"
	"testing"
)

func TestURLValidator(t *testing.T) {
	mockResolver := func(host string) ([]net.IP, error) {
		switch host {
		case "google.com", "github.com":
			return []net.IP{net.ParseIP("142.250.190.46")}, nil
		case "intranet.local":
			return []net.IP{net.ParseIP("192.168.0.50")}, nil
		default:
			return nil, net.UnknownNetworkError("host not found")
		}
	}

	validator := NewURLValidatorWithResolver([]string{"phishingsite.com", "malware.ru"}, mockResolver)

	tests := []struct {
		url       string
		shouldErr bool
	}{
		{"http://localhost:8080/secret", true},
		{"http://127.0.0.1/admin", true},
		{"http://192.168.1.1/router", true},
		{"http://10.0.0.1/internal", true},
		{"http://intranet.local/dashboard", true},
		{"https://phishingsite.com/login", true},
		{"https://sub.malware.ru/exe", true},
		{"ftp://ftp.example.com", true},
		{"https://google.com", false},
		{"https://github.com", false},
	}

	for _, tt := range tests {
		err := validator.ValidateDestination(tt.url)
		if tt.shouldErr && err == nil {
			t.Errorf("Esperava erro para URL '%s', mas passou", tt.url)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("Não esperava erro para URL '%s', mas recebeu: %v", tt.url, err)
		}
	}
}
