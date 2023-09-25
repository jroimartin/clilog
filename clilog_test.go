package clilog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2023, time.September, 20, 12, 24, 43, 0, time.UTC)

func TestCLIHandler(t *testing.T) {
	tests := []struct {
		name  string
		opts  *HandlerOptions
		with  func(*slog.Logger) *slog.Logger
		attrs []slog.Attr
		want  string
	}{
		{
			name:  "basic",
			attrs: []slog.Attr{slog.String("c", "foo"), slog.Bool("b", true)},
			want:  `2023-09-20T12:24:43Z INFO message c=foo b=true`,
		},
		{
			name: "group",
			attrs: []slog.Attr{
				slog.String("c", "foo"),
				slog.Group("g", slog.Int("a", 1), slog.Int("d", 4)),
				slog.Bool("b", true),
			},
			want: `2023-09-20T12:24:43Z INFO message c=foo g.a=1 g.d=4 b=true`,
		},
		{
			name:  "source",
			opts:  &HandlerOptions{AddSource: true},
			attrs: []slog.Attr{slog.String("c", "foo"), slog.Bool("b", true)},
			want:  `2023-09-20T12:24:43Z INFO $SOURCE message c=foo b=true`,
		},
		{
			name: "WithAttrs",
			with: func(l *slog.Logger) *slog.Logger {
				return l.With("wa", 1, "wb", 2)
			},
			attrs: []slog.Attr{slog.String("c", "foo"), slog.Bool("b", true)},
			want:  `2023-09-20T12:24:43Z INFO message wa=1 wb=2 c=foo b=true`,
		},
		{
			name: "WithAttrs,WithGroup",
			with: func(l *slog.Logger) *slog.Logger {
				return l.With("wa", 1, "wb", 2).WithGroup("p1").With("wc", 3).WithGroup("p2")
			},
			attrs: []slog.Attr{slog.String("c", "foo"), slog.Bool("b", true)},
			want:  `2023-09-20T12:24:43Z INFO message wa=1 wb=2 p1.wc=3 p1.p2.c=foo p1.p2.b=true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			h := NewCLIHandler(&buf, tt.opts)
			logger := slog.New(setTimeHandler{testTime, h})

			if tt.with != nil {
				logger = tt.with(logger)
			}

			logger.LogAttrs(context.Background(), slog.LevelInfo, "message", tt.attrs...)
			_, file, line, ok := runtime.Caller(0)
			if !ok {
				t.Fatalf("could not get source line")
			}

			// The call to tt.logf happens one line before
			// calling runtime.Caller.
			source := fmt.Sprintf("%v:%v", file, line-1)

			want := strings.ReplaceAll(tt.want, "$SOURCE", source)
			if got := strings.TrimSuffix(buf.String(), "\n"); got != want {
				t.Errorf("\ngot  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestCLIHandler_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		minLevel slog.Level
		level    slog.Level
		want     bool
	}{
		{
			name:     "warn debug",
			minLevel: slog.LevelWarn,
			level:    slog.LevelDebug,
			want:     false,
		},
		{
			name:     "warn info",
			minLevel: slog.LevelWarn,
			level:    slog.LevelInfo,
			want:     false,
		},
		{
			name:     "warn warn",
			minLevel: slog.LevelWarn,
			level:    slog.LevelWarn,
			want:     true,
		},
		{
			name:     "warn error",
			minLevel: slog.LevelWarn,
			level:    slog.LevelError,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewCLIHandler(io.Discard, &HandlerOptions{Level: tt.minLevel})
			if got := h.Enabled(context.Background(), tt.level); got != tt.want {
				t.Errorf("*CLIHandler.Enabled returned an unexpected value: got: %v, want: %v", got, tt.want)
			}
		})
	}
}

type setTimeHandler struct {
	t time.Time
	h slog.Handler
}

func (h setTimeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h setTimeHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Time = h.t
	return h.h.Handle(ctx, r)
}

func (h setTimeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return setTimeHandler{t: h.t, h: h.h.WithAttrs(attrs)}
}

func (h setTimeHandler) WithGroup(name string) slog.Handler {
	return setTimeHandler{t: h.t, h: h.h.WithGroup(name)}
}
