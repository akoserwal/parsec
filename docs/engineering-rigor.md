# Engineering Rigor: A Practical Course

> "Rather than add comments, I'm just pushing some changes."
>
> The highest form of code review is code itself.

This guide distills engineering rigor into teachable principles, drawn from
real-world Go service development. Each chapter covers one principle with the
**why**, the **anti-pattern**, the **rigorous pattern**, and exercises you can
apply to your own codebase.

---

## Table of Contents

1. [Explicit Over Implicit](#1-explicit-over-implicit)
2. [Dependency Injection for I/O](#2-dependency-injection-for-io)
3. [In-Memory Transports: Eliminate the Socket](#3-in-memory-transports-eliminate-the-socket)
4. [Eliminating False Test Boundaries](#4-eliminating-false-test-boundaries)
5. [Goroutine Lifecycle Discipline](#5-goroutine-lifecycle-discipline)
6. [Fail Fast with Clear Errors](#6-fail-fast-with-clear-errors)
7. [Resource Leak Prevention](#7-resource-leak-prevention)
8. [Design for the Consumer, Not the Default](#8-design-for-the-consumer-not-the-default)
9. [Comments That Earn Their Keep](#9-comments-that-earn-their-keep)
10. [Code as Review Feedback](#10-code-as-review-feedback)
11. [Knowing When to Stop](#11-knowing-when-to-stop)

---

## 1. Explicit Over Implicit

### The Principle

When a library gives you a convenient default, and your correctness depends on
that default, make it explicit. Defaults can change. Readers don't know what
they don't see.

### Anti-Pattern

```go
hs := health.NewServer()
// The library sets "" to SERVING internally.
// We trust that. We don't mention it.

// Later, a reader asks: "Is the empty-string service registered?
// Who sets it? When? To what?"
```

The code is correct but opaque. A reader must know the library's internals to
understand the system's behavior. The implicit default is a hidden dependency
on implementation details.

### Rigorous Pattern

```go
hs := health.NewServer()

// Even though NewServer sets "" to SERVING by default, be explicit.
// This makes the behavior visible in our code and decouples us from
// library implementation details.
hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
hs.SetServingStatus("readiness", healthpb.HealthCheckResponse_NOT_SERVING)
for _, svc := range healthServices {
    hs.SetServingStatus(svc, healthpb.HealthCheckResponse_NOT_SERVING)
}
```

Now the code reads as a complete specification. Every registered service name,
every initial status — all visible in one place. If the library changes its
default, your code still does what you intend.

### The Test for This Principle

Ask: **"If I deleted the library's source code, could a reader still understand
what this code does?"** If not, add explicit calls.

### When to Break This Rule

Don't be explicit about things that are truly irrelevant to your domain. You
don't need to write `http.StatusOK` when the default zero-value behavior is
fine. The line is: **does your correctness depend on it?** If yes, be explicit.

### Exercise

Find three places in your codebase where behavior depends on a library default.
Add explicit calls. Write a comment explaining why.

---

## 2. Dependency Injection for I/O

### The Principle

Code that creates its own I/O resources (opens sockets, creates files, starts
timers) is code that can only run in one environment. Code that accepts I/O
resources from the caller can run in any environment.

### Anti-Pattern

```go
type Config struct {
    GRPCPort int
    HTTPPort int
}

func (s *Server) Start(ctx context.Context) error {
    grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.grpcPort))
    if err != nil {
        return err
    }
    // The server now owns socket creation.
    // Tests must use real TCP ports.
    // CI environments fight over port allocation.
    // You can't test without the network stack.
}
```

The server has two responsibilities: managing sockets AND serving requests. That
coupling makes the server untestable without the operating system's network
stack.

### Rigorous Pattern

```go
type Config struct {
    GRPCListener    net.Listener
    HTTPListener    net.Listener
    GRPCDialOptions []grpc.DialOption
}

func (s *Server) Start(ctx context.Context) error {
    // Server only serves. Caller decides how connections arrive.
    s.grpcServer = grpc.NewServer()
    // ...
    go s.grpcServer.Serve(s.grpcListener)
}
```

Production code creates TCP listeners:

```go
grpcLis, _ := net.Listen("tcp", ":9066")
httpLis, _ := net.Listen("tcp", ":8080")
srv := server.New(server.Config{
    GRPCListener: grpcLis,
    HTTPListener: httpLis,
})
```

Test code creates in-memory listeners:

```go
grpcBuf := bufconn.Listen(1 << 20)
httpBuf := bufconn.Listen(1 << 20)
srv := server.New(server.Config{
    GRPCListener: grpcBuf,
    HTTPListener: httpBuf,
    GRPCDialOptions: []grpc.DialOption{
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return grpcBuf.DialContext(ctx)
        }),
    },
})
```

Same server. Same code path. No sockets.

### The Deeper Lesson

This is the Dependency Inversion Principle applied to infrastructure. The
server depends on the *abstraction* (`net.Listener`), not the *concrete
implementation* (TCP socket). The caller — whether production main or test
harness — provides the implementation.

The `GRPCDialOptions` field exists because grpc-gateway needs to dial back into
the gRPC server, and that dial path also needs to be injectable. When you find
yourself needing to inject one thing, look for **all** the I/O paths and inject
those too.

### Exercise

1. Find a component in your codebase that opens its own socket/file/connection.
2. Refactor it to accept the resource as a parameter.
3. Write a test using an in-memory substitute.

---

## 3. In-Memory Transports: Eliminate the Socket

### The Principle

Go gives you all the tools to avoid real network I/O in tests. A test that
opens a TCP port is slower, flakier, and harder to parallelize than one using
an in-memory pipe. Use `bufconn` for gRPC and custom `http.Transport` dialers
for HTTP.

### The Toolkit

**gRPC: `google.golang.org/grpc/test/bufconn`**

```go
buf := bufconn.Listen(1 << 20) // 1 MiB buffer

// Server side: serve on the bufconn listener
grpcServer.Serve(buf)

// Client side: dial through the bufconn
conn, _ := grpc.NewClient(
    "passthrough:///bufnet",
    grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
        return buf.DialContext(ctx)
    }),
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

**HTTP: Custom Transport Dialer**

```go
httpBuf := bufconn.Listen(1 << 20)

// Server side
httpServer := &http.Server{Handler: mux}
go httpServer.Serve(httpBuf)

// Client side
httpClient := &http.Client{
    Transport: &http.Transport{
        DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
            return httpBuf.DialContext(ctx)
        },
    },
}

// Now httpClient.Get("http://bufnet/healthz/live") routes through memory
```

### Why This Matters

| Concern | TCP Socket | bufconn |
|---------|-----------|---------|
| Port conflicts in CI | Yes | Impossible |
| OS firewall interference | Possible | Impossible |
| Parallelizable with `-count=100` | Risky | Safe |
| Time to establish connection | ~1ms | ~0ms |
| Tests the real server stack | Yes | Yes |
| Tests the real HTTP routing | Yes | Yes |

The critical insight: **bufconn tests are not "less real" than TCP tests.** The
gRPC server, HTTP server, grpc-gateway, and all your handlers execute exactly
the same code paths. The only thing you're skipping is the kernel's TCP stack,
which is not your code and not what you're testing.

### When You DO Need TCP

- Testing TLS certificate handling
- Testing connection timeouts and network partitions
- Load/performance testing
- Testing firewall/proxy behavior

For everything else, stay in memory.

### Exercise

Take an integration test that opens a real port. Convert it to use bufconn.
Measure the speed difference with `go test -bench`.

---

## 4. Eliminating False Test Boundaries

### The Principle

The distinction between "unit test" and "integration test" should reflect a
real architectural boundary, not an arbitrary choice about whether you open a
socket. If your in-memory test exercises the same code as your TCP test, you
don't need both.

### Anti-Pattern: Two Tests for One Thing

```
internal/server/health_test.go          (unit: httptest.ResponseRecorder)
test/integration/health_test.go         (integration: real TCP socket)
```

The "unit test" creates a bare `Server{healthServer: hs}` and calls
`handleReadiness` directly. The "integration test" starts a full server on a
real port, dials a client, and calls the same endpoint through HTTP. Both test
that the readiness handler returns 503 when services aren't SERVING.

The unit test is **less thorough** (it skips routing, mux behavior, and
middleware). The integration test is **slower and flakier** (it uses real TCP).
Neither is ideal.

### Rigorous Pattern: One Comprehensive Test

```go
func TestHealthLifecycle(t *testing.T) {
    env := startTestServer(t, stubServerConfig()) // bufconn, full stack

    // Test the actual endpoint through the actual HTTP server.
    // No sockets, no port conflicts, but exercises the full code path
    // including routing, mux, and handler.
    resp, _ := env.HTTPClient.Get(env.HTTPBaseURL + "/healthz/ready")
    // ...
}
```

This single test:
- Tests real HTTP routing (Go 1.22 method-pattern `"GET /healthz/ready"`)
- Tests the actual `http.ServeMux` dispatch
- Tests the handler logic
- Tests JSON serialization
- Tests response headers
- Runs in memory, no TCP

The "unit" tests that called handlers directly are now redundant.

### The Decision Framework

Ask these questions:

1. **Does the "unit test" exercise code paths the "integration test" misses?**
   If not, the unit test is redundant.

2. **Does the "integration test" catch bugs the "unit test" doesn't?**
   If yes, the unit test gives false confidence.

3. **Can I make the integration test as fast as the unit test?**
   With bufconn, usually yes. If so, keep only the integration test.

### When Separate Levels Are Justified

- **External services**: Tests against a real database vs. a mock are genuinely
  different.
- **Multi-process**: Testing that two separate binaries communicate correctly.
- **Performance**: Unit tests for logic, load tests for throughput.
- **Truly isolated logic**: A pure function that parses a string doesn't need
  a server.

### Exercise

Find a unit/integration test pair in your codebase that tests the same behavior.
Merge them into one test using in-memory transports.

---

## 5. Goroutine Lifecycle Discipline

### The Principle

Never launch a goroutine before all fallible setup is complete. If setup fails
and returns an error, every running goroutine becomes an orphan — still
executing, still holding resources, with no one to stop it.

### Anti-Pattern

```go
func (s *Server) Start(ctx context.Context) error {
    s.grpcServer = grpc.NewServer()
    // Register services...

    // Launch goroutine BEFORE grpc-gateway setup
    go s.grpcServer.Serve(s.grpcListener)

    // If this fails, the goroutine above is orphaned.
    if err := registerGatewayHandlers(ctx, gwMux, endpoint, opts); err != nil {
        return fmt.Errorf("failed to register handler: %w", err)
    }

    go s.httpServer.Serve(s.httpListener)
    return nil
}
```

If `registerGatewayHandlers` fails, `Start` returns an error, but the gRPC
serve goroutine is already running. The caller receives an error and reasonably
assumes nothing is running. The goroutine leaks.

### Rigorous Pattern

```go
func (s *Server) Start(ctx context.Context) error {
    s.grpcServer = grpc.NewServer()
    // Register services...

    // All fallible setup first
    if err := registerGatewayHandlers(ctx, gwMux, endpoint, opts); err != nil {
        return fmt.Errorf("failed to register handler: %w", err)
    }

    s.httpServer = &http.Server{Handler: httpMux}

    // All fallible setup is complete. Launch serve goroutines LAST so
    // that an early-return error never orphans a running goroutine.
    go s.grpcServer.Serve(s.grpcListener)
    go s.httpServer.Serve(s.httpListener)

    return nil
}
```

### The General Rule

Structure your `Start` function in three phases:

```
Phase 1: Allocate and configure (no side effects)
Phase 2: Validate and connect (fallible, may return error)
Phase 3: Launch background work (only if phases 1-2 succeeded)
```

This applies beyond goroutines. Any side effect — writing to a database,
publishing to a queue, modifying global state — should happen only after all
validation passes.

### The Shutdown Mirror

If `Start` has a three-phase structure, `Stop` should be its mirror:

```
Phase 1: Signal health watchers (healthServer.Shutdown)
Phase 2: Drain in-flight work (grpcServer.GracefulStop)
Phase 3: Close transport (httpServer.Shutdown)
```

```go
func (s *Server) Stop(ctx context.Context) error {
    // 1. Tell watchers we're going away
    if s.healthServer != nil {
        s.healthServer.Shutdown()
    }
    // 2. Drain in-flight RPCs
    if s.grpcServer != nil {
        s.grpcServer.GracefulStop()
    }
    // 3. Close HTTP transport
    if s.httpServer != nil {
        return s.httpServer.Shutdown(ctx)
    }
    return nil
}
```

### Exercise

Audit every `Start`/`Init`/`Run` function in your codebase. Draw a line
between "fallible setup" and "background launch." Are any goroutines launched
above that line?

---

## 6. Fail Fast with Clear Errors

### The Principle

When a function receives an input that will inevitably cause a failure later
(a nil pointer, an empty required field), fail immediately with a descriptive
error. Don't let the failure cascade into a confusing panic or a misleading
error message three call frames deep.

### Anti-Pattern

```go
func (s *Server) Start(ctx context.Context) error {
    s.grpcServer = grpc.NewServer()
    // ...

    // If s.grpcListener is nil, this panics with:
    //   "runtime error: invalid memory address or nil pointer dereference"
    // at net.(*TCPListener).Addr()
    // The reader must trace the stack to understand WHY it's nil.
    endpoint := "passthrough:///" + s.grpcListener.Addr().String()
}
```

### Rigorous Pattern

```go
func (s *Server) Start(ctx context.Context) error {
    if s.grpcListener == nil {
        return fmt.Errorf("missing gRPC listener")
    }
    if s.httpListener == nil {
        return fmt.Errorf("missing HTTP listener")
    }
    // Now all code below can assume non-nil listeners.
}
```

### Guard Clauses as Documentation

Guard clauses at the top of a function serve double duty:

1. **Runtime safety**: Prevent panics and cascading failures.
2. **Documentation**: They tell the reader what the function's preconditions
   are, without needing a comment.

```go
func (s *Server) Start(ctx context.Context) error {
    if s.grpcListener == nil {
        return fmt.Errorf("missing gRPC listener")
    }
    if s.httpListener == nil {
        return fmt.Errorf("missing HTTP listener")
    }
    // A reader now knows: Start requires both listeners.
    // No docstring needed.
}
```

### Test the Guards

Every guard clause deserves a test:

```go
func TestStartRejectsNilGRPCListener(t *testing.T) {
    srv := New(stubServerConfig())
    err := srv.Start(context.Background())
    if err == nil {
        t.Fatal("expected error when gRPC listener is nil")
    }
    if !strings.Contains(err.Error(), "missing gRPC listener") {
        t.Errorf("unexpected error: %v", err)
    }
}
```

This ensures the guard clause actually fires and produces the intended message.
Without the test, a future refactor might move or remove the guard.

### Exercise

Find a function that panics on nil input. Add a guard clause and a test.

---

## 7. Resource Leak Prevention

### The Principle

Every resource you acquire must have a corresponding release on every code
path, including error paths. In Go, `defer` is your primary tool, but you must
also think about **what owns the resource** and **when ownership transfers**.

### Anti-Pattern: Close on Success, Leak on Error

```go
grpcLis, err := net.Listen("tcp", ":9066")
if err != nil {
    return err
}

httpLis, err := net.Listen("tcp", ":8080")
if err != nil {
    // grpcLis is leaked! Nobody closes it on this error path.
    return err
}
```

### Rigorous Pattern: Defer Immediately

```go
grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
if err != nil {
    return fmt.Errorf("failed to listen on gRPC port: %w", err)
}
defer func() { _ = grpcLis.Close() }()

httpLis, err := net.Listen("tcp", fmt.Sprintf(":%d", httpPort))
if err != nil {
    return fmt.Errorf("failed to listen on HTTP port: %w", err)
}
defer func() { _ = httpLis.Close() }()
```

### The `_ =` Pattern

```go
defer func() { _ = grpcLis.Close() }()
```

Why `_ =` instead of just `defer grpcLis.Close()`? Two reasons:

1. **Linter compliance**: Many linters flag unhandled `Close()` errors. The
   explicit `_ =` says "I know this returns an error and I'm intentionally
   discarding it."
2. **Clarity of intent**: A future reader knows you considered the error and
   decided it's not actionable (which is usually correct for cleanup).

### In Tests: Use `t.Cleanup`

```go
grpcBuf := bufconn.Listen(1 << 20)
t.Cleanup(func() { _ = grpcBuf.Close() })

httpBuf := bufconn.Listen(1 << 20)
t.Cleanup(func() { _ = httpBuf.Close() })
```

`t.Cleanup` is better than `defer` in tests because:
- Cleanup functions run after the test and ALL its subtests complete
- Multiple cleanups run in LIFO order (like defer)
- They run even if the test calls `t.Fatal`

### Ownership Transfer

When you pass a listener to a server, who owns closing it?

```go
// serve.go creates the listener...
grpcLis, _ := net.Listen("tcp", ":9066")
defer func() { _ = grpcLis.Close() }()

// ...and passes it to the server
srv := server.New(server.Config{GRPCListener: grpcLis})
srv.Start(ctx)

// serve.go still owns cleanup via defer.
// The server uses the listener but doesn't close it.
```

This is a deliberate ownership decision. The creator closes; the borrower
uses. Document this in your `Config` type if it's not obvious.

### Exercise

Search your codebase for `net.Listen`, `os.Open`, `sql.Open`, or any resource
acquisition. For each one, verify there's a corresponding close on every code
path.

---

## 8. Design for the Consumer, Not the Default

### The Principle

When designing an API (a struct, a function, a service), think about what your
consumers need, not what's easiest to implement. A small inconvenience for the
implementer often translates to a large improvement for every consumer.

### Case Study: The `"readiness"` Service Key

The original implementation registered individual gRPC service names and used
the HTTP readiness endpoint to iterate over all of them. This works, but
consider the consumer: Kubernetes.

Kubernetes gRPC readiness probes check a single service name:

```yaml
readinessProbe:
  grpc:
    service: "readiness"
```

Kubernetes cannot be configured to check three services and AND the results.
Without a single aggregate key, the infrastructure team must either:
- Pick one service name arbitrarily (fragile, misleading)
- Write a custom script that checks all three (unnecessary complexity)
- Use the HTTP endpoint instead (loses the native gRPC probe benefit)

Adding a `"readiness"` aggregate key costs one line of code in the server but
saves every consumer from working around the limitation.

### The Wildcard Address Fix

Another consumer-oriented fix: `grpcDialEndpoint()`. The listener might
report its address as `0.0.0.0:8080`, which is a valid listen address but NOT
a valid dial address. The grpc-gateway is the consumer — it needs to dial back.
Rather than documenting "don't listen on 0.0.0.0," fix it at the source:

```go
func grpcDialEndpoint(listenAddr string) string {
    host, port, err := net.SplitHostPort(listenAddr)
    if err != nil {
        return "passthrough:///" + listenAddr
    }
    switch host {
    case "", "0.0.0.0":
        host = "127.0.0.1"
    case "::":
        host = "::1"
    }
    return "passthrough:///" + net.JoinHostPort(host, port)
}
```

This function asks: "What does my consumer need?" The consumer needs a dialable
address. The function translates the listen address into one.

### Exercise

Find an API in your codebase where callers must do extra work because the API
returns raw internal data instead of processed consumer-ready data. Refactor it.

---

## 9. Comments That Earn Their Keep

### The Principle

A comment must tell the reader something the code cannot. If the comment
restates the code, delete it. If the comment explains WHY a non-obvious
decision was made, keep it.

### Comments That Should Be Deleted

```go
// Create gRPC server
s.grpcServer = grpc.NewServer()

// Register services
authv3.RegisterAuthorizationServer(s.grpcServer, s.authzServer)
```

These comments say what the code already says. They add noise without signal.

### Comments That Earn Their Keep

```go
// Per the spec, register all services manually including the empty string
// for overall server health (liveness). The empty string is set to SERVING
// immediately and remains so until Shutdown() is called during Stop(),
// which sets every service to NOT_SERVING. Per-service and readiness
// statuses start as NOT_SERVING until SetReady() is called.
s.healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
```

This comment earns its keep because:
1. It references an external spec (the reader can look it up)
2. It explains the lifecycle (`SERVING` until `Shutdown()`)
3. It clarifies the relationship between `""`, per-service, and readiness keys
4. None of this is obvious from the code alone

### Another Example

```go
// All fallible setup is complete. Launch the serve goroutines last so
// that an early-return error never orphans a running goroutine.
go s.grpcServer.Serve(s.grpcListener)
```

This explains WHY the goroutine is here (at the bottom) instead of where you
might expect it (right after server creation). The structural decision is
intentional, and without the comment a future developer might "clean up" by
moving the goroutine earlier — reintroducing the leak.

### The Litmus Test

Before writing a comment, ask:

1. **Would a competent Go developer understand this code without the comment?**
   If yes, delete it.
2. **Does the comment explain WHY, not WHAT?** If it explains what, the code
   should be clearer instead.
3. **Does the comment reference external knowledge** (a spec, a design
   decision, a non-obvious constraint)? If yes, keep it.
4. **Would removing the comment risk a future developer making a mistake?**
   If yes, keep it.

### Exercise

Review 10 comments in your codebase. For each, ask the four questions above.
Delete the ones that fail.

---

## 10. Code as Review Feedback

### The Principle

When reviewing code, showing is more effective than telling. A code change
communicates the exact intent, demonstrates that it compiles and passes tests,
and leaves no room for misinterpretation. A comment says "you should do X."
A commit says "here is X, and it works."

### When to Use Code-as-Feedback

- The change is clear enough to be self-explanatory
- You can demonstrate the improvement rather than describe it
- The codebase benefits from the change regardless of the review discussion
- The change is small enough to review in return

### When to Use Written Feedback Instead

- The change involves a fundamental design disagreement that needs discussion
- The reviewer is unsure whether the change is correct
- The author needs to understand the principle, not just the fix
- The change is too large to digest as a surprise

### The Hybrid Approach

Alec's approach was ideal: he pushed code changes accompanied by a brief
description of his reasoning. Not just "here are changes" but "here's what
I did and why":

> "I don't like the separate 'not integration' vs 'integration' test. Go
> gives you all the tools to avoid this: grpc and http servers can both use
> in-memory transports. So I refactored the server to allow providing the
> listeners themselves and then collapsed the tests."

The code shows WHAT. The message shows WHY. Together they teach.

### Exercise

Next time you write a review comment longer than 3 sentences, consider: could
you express this as a commit instead?

---

## 11. Knowing When to Stop

### The Principle

Not every improvement is worth making. Engineering rigor includes the
discipline to recognize when a design is "good enough" and when further
improvement adds complexity without proportional value.

### Case Study: The Observer/Probe Pattern

Alec considered adding an observer pattern so that tests could synchronize
with the server's readiness transition without calling `SetReady()` directly:

```go
// Hypothetical observer hook
type ReadinessObserver func(ready bool)

type Config struct {
    OnReadinessChange ReadinessObserver
}
```

Tests would register an observer, start the server, and wait for the callback
instead of manually calling `SetReady()`. This would move `SetReady` out of
test code and into the production path.

He decided against it:

> "I decided to not do the last one, since maybe it makes some sense to
> invert control there."

Why stop? Because the current approach has a genuine advantage: **tests
explicitly control lifecycle phases.** A test that calls `SetReady()` itself
can assert behavior at each phase boundary. An observer pattern would make
tests more "black box" but also more complex and harder to debug when they fail.

### The Decision Framework

Before making an improvement, ask:

1. **Does the current code have a bug?** Fix it.
2. **Does the current code have a maintainability problem that will cause
   future bugs?** Fix it.
3. **Does the improvement make the code measurably better for its consumers?**
   Make it.
4. **Does the improvement add complexity proportional to its benefit?**
   If the complexity outweighs the benefit, stop.
5. **Will a future reader thank me for this change or be confused by it?**
   If confused, stop.

### The Meta-Principle

Engineering rigor is not about maximizing abstractions or applying every
pattern you know. It's about making **deliberate decisions** — including the
decision that the code is good enough.

The most rigorous thing you can do is stop improving and ship.

---

## Appendix A: Checklist

Use this checklist during code review or self-review:

```
[ ] Every behavior the code depends on is explicit, not relying on library defaults
[ ] I/O resources (sockets, files) are injected, not created internally
[ ] Tests use in-memory transports where possible
[ ] No false unit/integration boundary — one test level per behavior
[ ] Goroutines launch only after all fallible setup completes
[ ] Start() and Stop() are structural mirrors
[ ] Nil/empty inputs fail fast with descriptive errors
[ ] Every resource has a corresponding cleanup on every code path
[ ] API returns consumer-ready data, not raw internals
[ ] Comments explain WHY, not WHAT
[ ] Every improvement has been weighed against the complexity it adds
```

## Appendix B: Go-Specific Tools

| Need | Tool | Package |
|------|------|---------|
| In-memory gRPC transport | bufconn | `google.golang.org/grpc/test/bufconn` |
| In-memory HTTP transport | Custom `http.Transport` dialer | `net/http` |
| HTTP handler testing | `httptest.NewRecorder` | `net/http/httptest` |
| Test cleanup | `t.Cleanup` | `testing` |
| gRPC health checking | `health.NewServer()` | `google.golang.org/grpc/health` |
| Structured subtests | `t.Run` | `testing` |
| Table-driven tests | `[]struct{...}` + `t.Run` | `testing` |
| Stub dependencies | Interface + stub implementation | Your code |

## Appendix C: Recommended Reading

- [gRPC Health Checking Protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md)
- [Kubernetes gRPC Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe)
- [Go bufconn package](https://pkg.go.dev/google.golang.org/grpc/test/bufconn)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [The Twelve-Factor App](https://12factor.net/) — especially factor III (Config) and factor VI (Processes)
