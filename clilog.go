// Package clilog provides a [slog.Handler] for command line tools.
package clilog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"
)

// CLIHandler implements a [slog.Handler] for command line tools. The
// output format of CLIHandler is designed to be human readable.
type CLIHandler struct {
	opts  HandlerOptions
	group string // preformatted group, ends with a dot
	attrs string // preformatted attrs, begins with a white space

	mu sync.Mutex
	w  io.Writer
}

// HandlerOptions are options for a [CLIHandler]. A zero HandlerOptions
// consists entirely of default values.
type HandlerOptions struct {
	// AddSource causes the handler to output the source code
	// position of the log statement.
	AddSource bool

	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels. If Level is
	// nil, the handler assumes LevelInfo. The handler calls
	// Level.Level for each record processed; to adjust the
	// minimum level dynamically, use a LevelVar.
	Level slog.Leveler
}

// NewCLIHandler returns a new [CLIHandler].
func NewCLIHandler(w io.Writer, opts *HandlerOptions) *CLIHandler {
	if opts == nil {
		opts = &HandlerOptions{}
	}
	return &CLIHandler{
		opts: *opts,
		w:    w,
	}
}

// Enabled reports whether the handler handles records at the given
// level. The handler ignores records whose level is lower.
func (h *CLIHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle handles the Record.
func (h *CLIHandler) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder
	if !r.Time.IsZero() {
		b.WriteString(r.Time.Format(time.RFC3339) + " ")
	}
	b.WriteString(r.Level.String() + " ")
	if h.opts.AddSource && r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		fmt.Fprintf(&b, "%v:%v ", f.File, f.Line)
	}
	b.WriteString(r.Message)
	b.WriteString(h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		h.appendAttr(&b, h.group, a)
		return true
	})
	b.WriteString("\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write([]byte(b.String()))
	return err
}

// WithAttrs returns a new Handler whose attributes consist of both
// the receiver's attributes and the arguments.
func (h *CLIHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var b strings.Builder
	for _, a := range attrs {
		h.appendAttr(&b, h.group, a)
	}
	return &CLIHandler{
		opts:  h.opts,
		group: h.group,
		attrs: h.attrs + b.String(),
		w:     h.w,
	}
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups.
func (h *CLIHandler) WithGroup(name string) slog.Handler {
	return &CLIHandler{
		opts:  h.opts,
		group: h.group + name + ".",
		attrs: h.attrs,
		w:     h.w,
	}
}

func (h *CLIHandler) appendAttr(w io.Writer, group string, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}

	if a.Value.Kind() != slog.KindGroup {
		fmt.Fprintf(w, " %v%v=%v", group, a.Key, a.Value)
		return
	}

	if a.Key != "" {
		group += a.Key + "."
	}
	for _, a := range a.Value.Group() {
		h.appendAttr(w, group, a)
	}
}
