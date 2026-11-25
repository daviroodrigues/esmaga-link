package observability

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[90m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiMag    = "\033[35m"
)

type prettyColorHandler struct {
	mu     *sync.Mutex
	out    io.Writer
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
}

func newPrettyColorHandler(out io.Writer, level slog.Leveler) *prettyColorHandler {
	return &prettyColorHandler{
		mu:    &sync.Mutex{},
		out:   out,
		level: level,
	}
}

func (h *prettyColorHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.level.Level()
}

func (h *prettyColorHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer

	ts := r.Time
	if !ts.IsZero() {
		buf.WriteString(ansiDim)
		buf.WriteString(ts.Format("15:04:05"))
		buf.WriteString(ansiReset)
		buf.WriteByte(' ')
	}

	levelTag, levelColor := formatLevel(r.Level)
	buf.WriteString(levelColor)
	buf.WriteString(levelTag)
	buf.WriteString(ansiReset)
	buf.WriteByte(' ')

	buf.WriteString(ansiBold)
	buf.WriteString(r.Message)
	buf.WriteString(ansiReset)

	for _, a := range h.attrs {
		writeAttr(&buf, h.groups, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&buf, h.groups, a)
		return true
	})

	buf.WriteByte('\n')

	h.mu.Lock()
	_, err := h.out.Write(buf.Bytes())
	h.mu.Unlock()
	return err
}

func (h *prettyColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(next, h.attrs)
	copy(next[len(h.attrs):], attrs)
	return &prettyColorHandler{
		mu:     h.mu,
		out:    h.out,
		level:  h.level,
		attrs:  next,
		groups: h.groups,
	}
}

func (h *prettyColorHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := make([]string, len(h.groups)+1)
	copy(next, h.groups)
	next[len(h.groups)] = name
	return &prettyColorHandler{
		mu:     h.mu,
		out:    h.out,
		level:  h.level,
		attrs:  h.attrs,
		groups: next,
	}
}

func formatLevel(lvl slog.Level) (string, string) {
	switch {
	case lvl < slog.LevelInfo:
		return "DBG", ansiMag
	case lvl < slog.LevelWarn:
		return "INF", ansiGreen
	case lvl < slog.LevelError:
		return "WRN", ansiYellow
	default:
		return "ERR", ansiRed + ansiBold
	}
}

func writeAttr(buf *bytes.Buffer, groups []string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		subGroups := groups
		if a.Key != "" {
			subGroups = append(append([]string{}, groups...), a.Key)
		}
		for _, sub := range a.Value.Group() {
			writeAttr(buf, subGroups, sub)
		}
		return
	}

	buf.WriteByte(' ')
	buf.WriteString(ansiCyan)
	for _, g := range groups {
		buf.WriteString(g)
		buf.WriteByte('.')
	}
	buf.WriteString(a.Key)
	buf.WriteByte('=')
	buf.WriteString(ansiReset)
	buf.WriteString(fmt.Sprint(a.Value.Any()))
}

func InitLogger(serviceName, instanceID string) {
	handler := newPrettyColorHandler(os.Stdout, slog.LevelInfo).WithAttrs([]slog.Attr{
		slog.String("service", serviceName),
		slog.String("instance", instanceID),
	})
	slog.SetDefault(slog.New(handler))
}
