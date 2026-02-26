package logging

import "context"

// noopLogger silently discards all log output. It is useful as a safe
// default when no logger has been explicitly configured and as a test
// double that produces no side effects.
type noopLogger struct{}

// NewNoop returns a Logger that discards all log records.
// Enabled always returns false, so callers guarding expensive attribute
// construction will short-circuit immediately.
func NewNoop() Logger { return noopLogger{} }

func (noopLogger) Log(context.Context, Level, string, ...Attr) {}
func (noopLogger) Enabled(context.Context, Level) bool         { return false }
func (n noopLogger) With(...Attr) Logger                       { return n }
func (n noopLogger) WithGroup(string) Logger                   { return n }
