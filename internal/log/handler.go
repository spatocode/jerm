package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/fatih/color"
)

type Handler struct {
	opts      slog.HandlerOptions
	prefix    string
	preformat string
	mu        *sync.Mutex
	w         io.Writer
}

func New(w io.Writer, opts *slog.HandlerOptions) *Handler {
	h := &Handler{w: w, mu: &sync.Mutex{}}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	return h
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	buf := make([]byte, 0, 1024)

	if !r.Time.IsZero() {
		buf = r.Time.AppendFormat(buf, time.RFC3339)
		buf = append(buf, ' ')
	}

	switch r.Level {
	case slog.LevelDebug:
		level = color.MagentaString(level)
	case slog.LevelInfo:
		level = color.CyanString(level)
	case slog.LevelWarn:
		level = color.YellowString(level)
	case slog.LevelError:
		level = color.RedString(level)
	}

	buf = append(buf, level...)
	buf = append(buf, ' ')
	if h.opts.AddSource && r.PC != 0 {
		fr := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fr.Next()
		buf = append(buf, f.File...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(f.Line), 10)
		buf = append(buf, ' ')
	}
	buf = append(buf, color.CyanString(r.Message)...)
	buf = append(buf, h.preformat...)
	r.Attrs(func(a slog.Attr) bool {
		buf = h.appendAttr(buf, h.prefix, a)
		return true
	})
	buf = append(buf, '\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func (h *Handler) appendAttr(buf []byte, prefix string, a slog.Attr) []byte {
	if a.Equal(slog.Attr{}) {
		return buf
	}
	if a.Value.Kind() != slog.KindGroup {
		buf = append(buf, ' ')
		buf = append(buf, prefix...)
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		return fmt.Appendf(buf, "%v", a.Value.Any())
	}

	if a.Key != "" {
		prefix += a.Key + "."
	}
	for _, a := range a.Value.Group() {
		buf = h.appendAttr(buf, prefix, a)
	}
	return buf
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var buf []byte
	for _, a := range attrs {
		buf = h.appendAttr(buf, h.prefix, a)
	}
	return &Handler{
		w:         h.w,
		opts:      h.opts,
		prefix:    h.prefix,
		preformat: h.preformat + string(buf),
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		w:         h.w,
		opts:      h.opts,
		preformat: h.preformat,
		prefix:    h.prefix + name + ".",
	}
}
