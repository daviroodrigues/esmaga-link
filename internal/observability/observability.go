package observability

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Metrics coleta métricas HTTP e de runtime no formato de exposição do Prometheus.
type Metrics struct {
	mu             sync.RWMutex
	requestsTotal  map[string]*atomic.Int64
	redirectsTotal atomic.Int64
	durationSumNs  atomic.Int64
	durationCount  atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		requestsTotal: make(map[string]*atomic.Int64),
	}
}

func (m *Metrics) IncRedirect() {
	m.redirectsTotal.Add(1)
}

func (m *Metrics) ObserveHTTP(method, routePattern string, status int, d time.Duration) {
	if routePattern == "" {
		routePattern = "unknown"
	}
	key := fmt.Sprintf(`method="%s",route="%s",status="%d"`, method, routePattern, status)

	m.mu.RLock()
	counter, ok := m.requestsTotal[key]
	m.mu.RUnlock()

	if !ok {
		m.mu.Lock()
		counter, ok = m.requestsTotal[key]
		if !ok {
			counter = &atomic.Int64{}
			m.requestsTotal[key] = counter
		}
		m.mu.Unlock()
	}

	counter.Add(1)
	m.durationSumNs.Add(d.Nanoseconds())
	m.durationCount.Add(1)
}

// HTTPMiddleware registra logs estruturados com request_id e alimenta as métricas Prometheus.
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		routePattern := ""
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			routePattern = rctx.RoutePattern()
		}
		if routePattern == "" {
			routePattern = r.URL.Path
		}

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		if m != nil {
			m.ObserveHTTP(r.Method, routePattern, status, duration)
		}

		slog.Info("http_request",
			slog.String("request_id", middleware.GetReqID(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Float64("duration_ms", float64(duration.Microseconds())/1000.0),
			slog.String("remote_ip", r.RemoteAddr),
		)
	})
}

// Handler expõe /metrics no formato texto padrão do Prometheus (v0.0.4).
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		var sb strings.Builder
		sb.WriteString("# HELP esmaga_http_requests_total Total de requisições HTTP processadas.\n")
		sb.WriteString("# TYPE esmaga_http_requests_total counter\n")

		m.mu.RLock()
		keys := make([]string, 0, len(m.requestsTotal))
		for k := range m.requestsTotal {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("esmaga_http_requests_total{%s} %d\n", k, m.requestsTotal[k].Load()))
		}
		m.mu.RUnlock()

		sb.WriteString("# HELP esmaga_http_request_duration_seconds_sum Soma da duração das requisições HTTP em segundos.\n")
		sb.WriteString("# TYPE esmaga_http_request_duration_seconds_sum counter\n")
		sb.WriteString(fmt.Sprintf("esmaga_http_request_duration_seconds_sum %.6f\n", float64(m.durationSumNs.Load())/1e9))

		sb.WriteString("# HELP esmaga_http_request_duration_seconds_count Total de requisições medidas.\n")
		sb.WriteString("# TYPE esmaga_http_request_duration_seconds_count counter\n")
		sb.WriteString(fmt.Sprintf("esmaga_http_request_duration_seconds_count %d\n", m.durationCount.Load()))

		sb.WriteString("# HELP esmaga_redirects_total Total de redirecionamentos executados.\n")
		sb.WriteString("# TYPE esmaga_redirects_total counter\n")
		sb.WriteString(fmt.Sprintf("esmaga_redirects_total %d\n", m.redirectsTotal.Load()))

		sb.WriteString("# HELP esmaga_go_goroutines Número de goroutines ativas.\n")
		sb.WriteString("# TYPE esmaga_go_goroutines gauge\n")
		sb.WriteString(fmt.Sprintf("esmaga_go_goroutines %d\n", runtime.NumGoroutine()))

		sb.WriteString("# HELP esmaga_go_mem_alloc_bytes Memória alocada em heap (bytes).\n")
		sb.WriteString("# TYPE esmaga_go_mem_alloc_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("esmaga_go_mem_alloc_bytes %d\n", mem.Alloc))

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sb.String()))
	}
}
