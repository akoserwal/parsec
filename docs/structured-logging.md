# Structured Logging

## Overview

Parsec uses a vendor-agnostic `Logger` interface for all structured logging in the infrastructure layer. The interface lives in `internal/logging/` and provides a single, context-aware logging model with strongly typed attributes, immutable logger instances, and an explicit level-check mechanism for performance-sensitive call sites.

Domain packages (`internal/service/`) never import `logging`. They use the existing Observer/Probe pattern for observability. The `Logger` interface is consumed only by infrastructure components (server, config, keys, probe bridge) and wired by the application layer (`cli/serve.go`).

## Design Goals

- **Single logging model** -- one `Log` method, no dual conventions
- **Context-first** -- every log call carries `context.Context` for future tracing and correlation
- **Strongly typed attributes** -- typed constructors prevent key-value mismatches
- **Immutable logger instances** -- `With` and `WithGroup` return new loggers, never mutate the parent
- **Vendor independence** -- swap slog for zap or zerolog by implementing one interface
- **Concurrency safe** -- all implementations must be safe for use by multiple goroutines

## Logger Interface

```go
package logging

import "context"

type Logger interface {
    Log(ctx context.Context, level Level, msg string, attrs ...Attr)
    Enabled(ctx context.Context, level Level) bool
    With(attrs ...Attr) Logger
    WithGroup(name string) Logger
}
```

### `Log` -- emit a structured record

`Log` is the single method for emitting log records at any level. The level is always explicit at the call site, making it easy to grep for all error-level logging or audit level usage during review.

```go
logger.Log(ctx, logging.Info, "parsec started",
    logging.String("grpc_addr", grpcAddr),
    logging.String("http_addr", httpAddr),
)
```

**Why context is mandatory:** Every function in Parsec that performs logging already has access to `context.Context` (server handlers, key rotation, config watching, probe callbacks). Making context mandatory means that when tracing and correlation IDs are added in the future, zero interface changes are required -- the context already flows through every call site.

**Why a single method instead of `Debug`/`Info`/`Warn`/`Error` helpers:** A single `Log` method eliminates the possibility of dual calling conventions (e.g. having both `logger.Info(msg)` and `logger.Log(ctx, Info, msg)`). It also makes the level visible and explicit at every call site, which aids code review and auditing.

### `Enabled` -- skip expensive work

`Enabled` reports whether the logger will actually emit a record at the given level. This lets callers avoid constructing attributes that would be immediately discarded.

```go
if logger.Enabled(ctx, logging.Debug) {
    logger.Log(ctx, logging.Debug, "details", expensiveAttr())
}
```

#### Two layers of protection

The logging system has two independent short-circuit mechanisms:

1. **Adapter-internal short-circuit.** The slog adapter's `Log` method checks `slog.Logger.Enabled()` before allocating the `[]slog.Attr` slice and converting attributes. This protects against the cost of attribute *conversion* inside the adapter.

2. **Caller-side `Enabled` guard.** The `Enabled` method on the interface lets the *caller* skip all work before `Log` is even called -- building attribute slices, calling `status.Code()`, computing durations, performing nil checks, etc.

The adapter-internal check is always active. The caller-side guard is opt-in for call sites where the pre-call work is non-trivial.

#### When to use `Enabled`

Use `Enabled` when the call site does meaningful work before calling `Log`:

| Call site | Work avoided by Enabled guard | Guard needed? |
|---|---|---|
| gRPC interceptor | `status.Code(err).String()`, `time.Since(start)`, 5+ attr allocations per request | Yes |
| HTTP middleware | status capture, duration computation, path extraction per request | Yes |
| Probe `TokenIssuanceStarted` | conditional slice building, nil checks on subject/actor, append calls | Yes |
| `server.go` error logging | just `Err(err)` -- trivially cheap | No |
| `loader.go` watch errors | just `Err(err)` -- trivially cheap | No |

**gRPC interceptor example:**

```go
func UnaryServerInterceptor(logger Logger) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        start := time.Now()
        resp, err := handler(ctx, req)

        // Without Enabled: attrs are always constructed even if level is filtered out
        // With Enabled: all of this is skipped when Info is disabled
        if logger.Enabled(ctx, Info) {
            logger.Log(ctx, Info, "grpc request",
                String("method", info.FullMethod),
                Duration("duration", time.Since(start)),
                String("status", status.Code(err).String()),
            )
        }

        return resp, err
    }
}
```

**Probe bridge example:**

```go
func (o *loggingObserver) TokenIssuanceStarted(
    ctx context.Context,
    subject *trust.Result,
    actor *trust.Result,
    scope string,
    tokenTypes []service.TokenType,
) (context.Context, service.TokenIssuanceProbe) {
    probeLogger := o.logger.With(logging.String("event", "token_issuance"))

    if probeLogger.Enabled(ctx, logging.Debug) {
        attrs := []logging.Attr{
            logging.String("scope", scope),
            logging.Any("token_types", tokenTypes),
        }
        if subject != nil {
            attrs = append(attrs,
                logging.String("subject_id", subject.Subject),
                logging.String("subject_trust_domain", subject.TrustDomain),
            )
        }
        if actor != nil {
            attrs = append(attrs,
                logging.String("actor_id", actor.Subject),
                logging.String("actor_trust_domain", actor.TrustDomain),
            )
        }
        probeLogger.Log(ctx, logging.Debug, "Starting token issuance", attrs...)
    }

    return ctx, &loggingTokenIssuanceProbe{ctx: ctx, logger: probeLogger}
}
```

**Simple error logging -- no guard needed:**

```go
// Err(err) is trivially cheap; the adapter's internal check is sufficient
logger.Log(ctx, logging.Error, "gRPC server error", logging.Err(err))
```

### `With` -- attach attributes to a child logger

`With` returns a new `Logger` instance that includes the given attributes in every subsequent log record. The parent logger is never mutated.

```go
reqLogger := logger.With(logging.String("request_id", reqID))
reqLogger.Log(ctx, logging.Info, "handling request")
// output includes: request_id=abc123
```

In the probe bridge, `With` is used to scope all probe logs under an event name:

```go
probeLogger := o.logger.With(logging.String("event", "token_issuance"))
```

Every log record emitted through `probeLogger` carries `"event": "token_issuance"`, which the event filtering handler uses for per-event level control.

### `WithGroup` -- namespace attributes

`WithGroup` returns a new `Logger` that nests all subsequent attributes under a named group. This produces structured JSON output with nested objects.

```go
logger.WithGroup("rotation").
    With(logging.String("slot", "primary")).
    Log(ctx, logging.Info, "key rotation completed")
```

Produces:

```json
{
  "msg": "key rotation completed",
  "rotation": {
    "slot": "primary"
  }
}
```

This is useful for grouping related attributes without key collisions (e.g. `request.method` vs `response.status`).

## Level

```go
type Level uint8

const (
    Debug Level = iota  // 0
    Info                // 1
    Warn                // 2
    Error               // 3
)
```

Levels use `uint8` with sequential `iota` values rather than matching any backend's numeric model. This prevents:

- Arithmetic misuse (e.g. `slog.LevelInfo + 2` to get Warn)
- Leaking slog's specific values (-4, 0, 4, 8) into the interface contract

Mapping to the concrete backend happens only inside the adapter:

```go
func toSlogLevel(l Level) slog.Level {
    switch l {
    case Debug: return slog.LevelDebug   // -4
    case Info:  return slog.LevelInfo    //  0
    case Warn:  return slog.LevelWarn    //  4
    case Error: return slog.LevelError   //  8
    default:    return slog.LevelInfo
    }
}
```

### Level usage guidelines

| Level | Use for | Examples in Parsec |
|---|---|---|
| `Debug` | Internal operational detail, high-volume events | Cache refresh ticks, probe lifecycle, health check requests |
| `Info` | Normal operational events | Server started, key rotated, request served |
| `Warn` | Recoverable issues that may need attention | Cache refresh failed (stale data served), config watch error |
| `Error` | Unrecoverable or unexpected failures | Server crash, key generation failure, missing listener |

## Attr

```go
type Attr struct {
    Key   string
    Value any
}
```

Typed constructors enforce correct usage at call sites:

| Constructor | Type | Example |
|---|---|---|
| `String(key, val)` | `string` | `String("method", "GET")` |
| `Int(key, val)` | `int` | `Int("status", 200)` |
| `Int64(key, val)` | `int64` | `Int64("bytes", contentLength)` |
| `Bool(key, val)` | `bool` | `Bool("ok", true)` |
| `Duration(key, val)` | `time.Duration` | `Duration("elapsed", time.Since(start))` |
| `Time(key, val)` | `time.Time` | `Time("issued_at", token.IssuedAt)` |
| `Any(key, val)` | `any` | `Any("token_types", tokenTypes)` |
| `Err(err)` | `error` | `Err(err)` -- key is always `"error"` |

`Value` is typed as `any` so that the logging package has no dependency on any backend. The slog adapter uses `slog.Any()` internally, which does a type-switch for `string`, `int64`, `float64`, `bool`, `time.Time`, `time.Duration`, and `error` -- so common types are encoded efficiently without reflection.

`Err(err)` standardises the error attribute key as `"error"` across the entire codebase, replacing the inconsistent `slog.String("error", err.Error())` pattern.

## Implementations

### slog adapter (`slog.go`)

The default adapter wraps `*slog.Logger` from Go's standard library (`log/slog`). This is the only file in `internal/logging/` that imports `log/slog`.

```go
logger := logging.NewFromSlog(slog.New(handler))
```

Key behaviours:

- `Log()` short-circuits via `slog.Logger.Enabled()` before allocating the `[]slog.Attr` conversion slice
- `With()` calls `handler.WithAttrs()` to create a new handler, preserving the parent's immutability
- `WithGroup()` delegates directly to `slog.Logger.WithGroup()`
- `NewFromSlog(nil)` returns `NewNoop()` as a safe fallback

### Noop logger (`noop.go`)

A logger that silently discards all output. `Enabled` always returns `false`.

```go
logger := logging.NewNoop()
```

Used as:

- Safe default when a `Logger` field is not explicitly configured (e.g. `if cfg.Logger == nil { logger = logging.NewNoop() }`)
- Test double that produces no side effects

## Architecture

```
Domain Layer (internal/service/)
    Observer/Probe interfaces -- pure, no logging imports

Infrastructure Layer
    internal/logging/        -- Logger interface + slog adapter
    internal/probe/          -- Implements Observer using Logger (bridge)
    internal/server/         -- Uses Logger for operational logging
    internal/keys/           -- Uses Logger for rotation logging
    internal/config/         -- Creates Logger, uses for config watch logging

Application Layer (internal/cli/)
    cli/serve.go             -- Wires Logger to all components
```

The domain layer never imports `logging`. Domain observability flows through the Observer/Probe pattern defined in `internal/service/observability.go`. The probe bridge (`internal/probe/logging.go`) implements those domain interfaces and translates events into structured log records using the `Logger` interface.
