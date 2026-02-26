package logging

import (
	"context"
	"log/slog"
)

// slogLogger adapts a *slog.Logger to the Logger interface.
// This is the only file in the logging package that imports log/slog.
type slogLogger struct {
	l *slog.Logger
}

// NewFromSlog wraps a *slog.Logger as a Logger.
// The returned Logger delegates all operations to the underlying slog.Logger
// and is safe for concurrent use.
func NewFromSlog(l *slog.Logger) Logger {
	if l == nil {
		return NewNoop()
	}
	return &slogLogger{l: l}
}

func (s *slogLogger) Log(ctx context.Context, level Level, msg string, attrs ...Attr) {
	sl := toSlogLevel(level)
	if !s.l.Enabled(ctx, sl) {
		return
	}
	slogAttrs := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		slogAttrs[i] = slog.Any(a.Key, a.Value)
	}
	s.l.LogAttrs(ctx, sl, msg, slogAttrs...)
}

func (s *slogLogger) Enabled(ctx context.Context, level Level) bool {
	return s.l.Enabled(ctx, toSlogLevel(level))
}

func (s *slogLogger) With(attrs ...Attr) Logger {
	if len(attrs) == 0 {
		return s
	}
	slogAttrs := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		slogAttrs[i] = slog.Any(a.Key, a.Value)
	}
	newHandler := s.l.Handler().WithAttrs(slogAttrs)
	return &slogLogger{l: slog.New(newHandler)}
}

func (s *slogLogger) WithGroup(name string) Logger {
	return &slogLogger{l: s.l.WithGroup(name)}
}

// toSlogLevel maps the vendor-agnostic Level to slog.Level.
func toSlogLevel(l Level) slog.Level {
	switch l {
	case Debug:
		return slog.LevelDebug
	case Info:
		return slog.LevelInfo
	case Warn:
		return slog.LevelWarn
	case Error:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
