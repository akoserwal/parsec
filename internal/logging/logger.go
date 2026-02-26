// Package logging provides a structured, vendor-agnostic logging abstraction.
//
// The Logger interface is the single entry point for all structured logging
// in the infrastructure layer. Domain packages must not import this package;
// they use the Observer/Probe pattern for observability instead.
//
// Concrete implementations (e.g. slog, zap, zerolog) live behind this
// interface and are wired at application startup.
package logging

import "context"

// Logger is a structured, context-aware, immutable logger.
//
// Implementations MUST be safe for concurrent use by multiple goroutines.
// Logger instances returned by With and WithGroup are new, immutable values
// that share no mutable state with the parent.
type Logger interface {
	// Log emits a structured log record at the given level.
	//
	// Context is mandatory to enable future correlation IDs, tracing, and
	// cancellation-aware logging without interface changes.
	Log(ctx context.Context, level Level, msg string, attrs ...Attr)

	// Enabled reports whether the logger will emit a record at the given level.
	// Callers should use this to skip expensive attribute construction:
	//
	//   if logger.Enabled(ctx, logging.Debug) {
	//       logger.Log(ctx, logging.Debug, "details", expensiveAttr())
	//   }
	Enabled(ctx context.Context, level Level) bool

	// With returns a new Logger that includes the given attributes in every
	// subsequent log record. The returned Logger is a new immutable instance.
	With(attrs ...Attr) Logger

	// WithGroup returns a new Logger that nests all subsequent attributes
	// under the named group. The returned Logger is a new immutable instance.
	WithGroup(name string) Logger
}
