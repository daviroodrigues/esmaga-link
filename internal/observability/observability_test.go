package observability

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrettyColorHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newPrettyColorHandler(&buf, slog.LevelInfo).WithAttrs([]slog.Attr{
		slog.String("service", "esmaga-api"),
	}))

	logger.Info("http_request", slog.String("method", "GET"), slog.Int("status", 200))

	out := buf.String()
	for _, want := range []string{"INF", "http_request", "service=", "esmaga-api", "method=", "GET", "status=", "200", ansiCyan} {
		if !strings.Contains(out, want) {
			t.Errorf("log formatado não contém %q: %q", want, out)
		}
	}
	_ = context.Background()
}

func TestMetrics_Handler(t *testing.T) {
	m := NewMetrics()
	m.ObserveHTTP(http.MethodGet, "/healthz", http.StatusOK, 5*time.Millisecond)
	m.IncRedirect()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`esmaga_http_requests_total{method="GET",route="/healthz",status="200"} 1`,
		`esmaga_redirects_total 1`,
		`esmaga_go_goroutines`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("saída de /metrics não contém %q", want)
		}
	}
}
