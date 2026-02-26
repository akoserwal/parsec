# Health Endpoints — Design & Code Flow

## Overview

Parsec implements the [gRPC Health Checking Protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md) as its single source of truth for health status, and exposes two additional HTTP endpoints for Kubernetes probe compatibility.

```
┌──────────────────────────────────────────────────────────────────┐
│                        Parsec Server                             │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │              health.Server  (single source of truth)        │ │
│  │                                                             │ │
│  │   ""  (empty)                          → SERVING (always)   │ │
│  │   envoy.service.auth.v3.Authorization  → NOT_SERVING / SERVING │
│  │   parsec.v1.TokenExchangeService       → NOT_SERVING / SERVING │
│  │   parsec.v1.JWKSService                → NOT_SERVING / SERVING │
│  └────────────┬──────────────────────────────┬─────────────────┘ │
│               │                              │                   │
│    ┌──────────▼──────────┐       ┌───────────▼──────────────┐    │
│    │   gRPC interface    │       │     HTTP interface        │    │
│    │   :9066 (default)   │       │     :8066 (default)       │    │
│    │                     │       │                           │    │
│    │  Health/Check       │       │  GET /healthz/live        │    │
│    │  Health/Watch       │       │  GET /healthz/ready       │    │
│    └─────────────────────┘       └───────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

## Interfaces

### gRPC — `grpc.health.v1.Health`

| RPC | Description |
|---|---|
| `Check(HealthCheckRequest)` | Returns the current `ServingStatus` for a named service. Returns gRPC `NOT_FOUND` for unregistered service names. |
| `Watch(HealthCheckRequest)` | Opens a server-streaming RPC. Immediately sends the current status, then pushes an update each time the status changes. |

Registered service names:

| Service key | Maps to |
|---|---|
| `""` (empty string) | Overall server liveness — always `SERVING` once the gRPC server is listening |
| `envoy.service.auth.v3.Authorization` | Envoy ext_authz service |
| `parsec.v1.TokenExchangeService` | OAuth 2.0 Token Exchange service |
| `parsec.v1.JWKSService` | JWKS endpoint service |

### HTTP

| Endpoint | Method | Purpose | Response |
|---|---|---|---|
| `/healthz/live` | `GET` | Liveness probe | Always `200 {"status":"OK"}` |
| `/healthz/ready` | `GET` | Readiness probe | `200 {"status":"SERVING"}` when all services are `SERVING`; `503 {"status":"NOT_SERVING","service":"<name>"}` otherwise |

Both HTTP handlers set `Cache-Control: no-cache, no-store, must-revalidate` to prevent stale responses through reverse proxies. Non-GET methods receive `405 Method Not Allowed`.

## Server Lifecycle

```
 ┌─────────┐
 │  New()   │  Server struct created, no listeners yet
 └────┬─────┘
      │
 ┌────▼─────────────────────────────────────────────────────────┐
 │  Start(ctx)                                                  │
 │                                                              │
 │  1. Create gRPC server                                       │
 │  2. Register application services (authz, exchange, jwks)    │
 │  3. Create health.Server                                     │
 │  4. Register health.Server on gRPC server                    │
 │  5. Set per-service status → NOT_SERVING                     │
 │  6. Register reflection service                              │
 │  7. Start gRPC listener (goroutine)                          │
 │  8. Create grpc-gateway mux + HTTP mux                       │
 │  9. Mount GET /healthz/live, GET /healthz/ready              │
 │ 10. Start HTTP listener (goroutine)                          │
 └────┬─────────────────────────────────────────────────────────┘
      │
      │  At this point:
      │    gRPC  Check("") → SERVING  (overall liveness ✓)
      │    gRPC  Check("parsec.v1.TokenExchangeService") → NOT_SERVING
      │    HTTP  /healthz/live → 200
      │    HTTP  /healthz/ready → 503
      │
 ┌────▼─────────────────────────────────────────────────────────┐
 │  SetReady()   (called from serve.go step 8a)                 │
 │                                                              │
 │  Sets all three per-service statuses → SERVING               │
 │  Watch streams receive a SERVING update                      │
 └────┬─────────────────────────────────────────────────────────┘
      │
      │  Now:
      │    gRPC  Check("parsec.v1.TokenExchangeService") → SERVING
      │    HTTP  /healthz/ready → 200  {"status":"SERVING"}
      │
      │  ═══════════ server is accepting traffic ═══════════
      │
 ┌────▼─────────────────────────────────────────────────────────┐
 │  SetNotReady()   (optional — for graceful drain)             │
 │                                                              │
 │  Sets all three per-service statuses → NOT_SERVING           │
 │  Watch streams receive a NOT_SERVING update                  │
 └────┬─────────────────────────────────────────────────────────┘
      │
 ┌────▼─────────────────────────────────────────────────────────┐
 │  Stop(ctx)                                                   │
 │                                                              │
 │  1. healthServer.Shutdown()                                  │
 │     → Sets ALL services to NOT_SERVING                       │
 │     → Ignores future SetServingStatus calls                  │
 │     → Watch streams see final NOT_SERVING before closing     │
 │  2. grpcServer.GracefulStop()                                │
 │     → Finishes in-flight RPCs, then stops                    │
 │  3. httpServer.Shutdown(ctx)                                 │
 │     → Finishes in-flight HTTP requests, then stops           │
 └──────────────────────────────────────────────────────────────┘
```

## Code Flow

### Where it lives

```
internal/server/server.go     ← Server struct, Start/Stop, SetReady/SetNotReady,
                                 handleLiveness, handleReadiness
internal/cli/serve.go          ← Calls srv.Start(), srv.SetReady(), srv.Stop()
```

### Readiness handler — detailed flow

```go
handleReadiness(w, r)
  │
  ├─ for each service in healthServices:
  │    │
  │    ├─ healthServer.Check(ctx, {Service: svc})
  │    │
  │    ├─ if error OR status ≠ SERVING:
  │    │    ├─ 503 {"status":"NOT_SERVING","service":"<svc>"}
  │    │    └─ return (fast-fail on first unhealthy service)
  │    │
  │    └─ continue to next service
  │
  └─ all services healthy:
       └─ 200 {"status":"SERVING"}
```

The readiness handler iterates over all three registered services. If **any** service returns something other than `SERVING`, it immediately returns `503` and identifies the specific failing service in the response body. This makes debugging straightforward — a `curl` to `/healthz/ready` tells you exactly which service is degraded.

## Kubernetes Configuration

### Using HTTP probes

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: parsec
          ports:
            - containerPort: 9066   # gRPC
              name: grpc
            - containerPort: 8066   # HTTP
              name: http
          livenessProbe:
            httpGet:
              path: /healthz/live
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /healthz/ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
```

### Using native gRPC probes (Kubernetes 1.24+)

```yaml
          livenessProbe:
            grpc:
              port: 9066
              # service: ""   (default — checks overall health)
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            grpc:
              port: 9066
              service: "parsec.v1.TokenExchangeService"
            initialDelaySeconds: 5
            periodSeconds: 5
```

> **Note**: Kubernetes native gRPC probes check one service at a time. To verify all three services, use the HTTP `/healthz/ready` endpoint which aggregates them.

## Verification

### Prerequisites

```bash
# Install grpcurl (if not already installed)
brew install grpcurl    # macOS
# or: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Start parsec
parsec serve --config config.yaml
```

### 1. HTTP Liveness

```bash
curl -s http://localhost:8066/healthz/live | jq .
```

Expected:
```json
{"status": "OK"}
```

### 2. HTTP Readiness

```bash
curl -s -w "\nHTTP %{http_code}\n" http://localhost:8066/healthz/ready | jq .
```

When ready:
```json
{"status": "SERVING"}
```
```
HTTP 200
```

When not ready (e.g. during startup or after SetNotReady):
```json
{"status": "NOT_SERVING", "service": "parsec.v1.JWKSService"}
```
```
HTTP 503
```

### 3. HTTP Method Restriction

```bash
curl -s -o /dev/null -w "HTTP %{http_code}\n" -X POST http://localhost:8066/healthz/live
```

Expected:
```
HTTP 405
```

### 4. gRPC — Overall Health (Liveness)

```bash
grpcurl -plaintext localhost:9066 grpc.health.v1.Health/Check
```

Expected:
```json
{
  "status": "SERVING"
}
```

### 5. gRPC — Per-Service Health (Readiness)

```bash
# Check a specific service
grpcurl -plaintext -d '{"service": "parsec.v1.TokenExchangeService"}' \
  localhost:9066 grpc.health.v1.Health/Check
```

Expected when ready:
```json
{
  "status": "SERVING"
}
```

Expected when not ready:
```json
{
  "status": "NOT_SERVING"
}
```

Check all three services:
```bash
for svc in \
  "envoy.service.auth.v3.Authorization" \
  "parsec.v1.TokenExchangeService" \
  "parsec.v1.JWKSService"; do
  echo "=== $svc ==="
  grpcurl -plaintext -d "{\"service\": \"$svc\"}" \
    localhost:9066 grpc.health.v1.Health/Check
done
```

### 6. gRPC — Unknown Service (Spec Compliance)

```bash
grpcurl -plaintext -d '{"service": "nonexistent.Service"}' \
  localhost:9066 grpc.health.v1.Health/Check
```

Expected:
```
ERROR:
  Code: NotFound
  Message: unknown service
```

### 7. gRPC — Watch (Streaming)

```bash
# This opens a streaming connection; leave it running and observe status changes.
grpcurl -plaintext -d '{"service": "parsec.v1.TokenExchangeService"}' \
  localhost:9066 grpc.health.v1.Health/Watch
```

The stream prints a JSON message each time the status changes:
```json
{
  "status": "NOT_SERVING"
}
{
  "status": "SERVING"
}
```

### 8. gRPC — List Services via Reflection

```bash
grpcurl -plaintext localhost:9066 list
```

Expected output includes:
```
envoy.service.auth.v3.Authorization
grpc.health.v1.Health
grpc.reflection.v1.ServerReflection
grpc.reflection.v1alpha.ServerReflection
parsec.v1.JWKSService
parsec.v1.TokenExchangeService
```

### 9. Cache-Control Header

```bash
curl -s -D - http://localhost:8066/healthz/ready | grep -i cache-control
```

Expected:
```
Cache-Control: no-cache, no-store, must-revalidate
```

## Test Coverage

| Scenario | Unit Test | Integration Test |
|---|:---:|:---:|
| Liveness always 200 | ✅ | ✅ |
| Readiness 503 before SetReady | ✅ | ✅ |
| Readiness 200 after SetReady | ✅ | ✅ |
| Readiness 503 after SetNotReady | — | ✅ |
| Readiness 503 after Shutdown | ✅ | — |
| Per-service failure identification | ✅ (×3) | — |
| gRPC Check("") overall health | — | ✅ |
| gRPC Check per-service NOT_SERVING | — | ✅ (×3) |
| gRPC Check per-service SERVING | — | ✅ (×3) |
| gRPC unknown service NOT_FOUND | — | ✅ |
| gRPC Health discoverable via reflection | — | ✅ |
| gRPC Watch streaming lifecycle | — | ✅ |
| HTTP GET-only method guard | via Go 1.22 mux | — |
| Cache-Control header | via handler impl | — |

### Running tests

```bash
# Unit tests only (fast, no network)
go test ./internal/server/ -run 'TestHandle' -v

# Integration tests (starts real servers)
go test ./test/integration/ -run 'TestHealth' -v

# Full test suite
go test ./... -count=1 -short
```
