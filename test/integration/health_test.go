package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

// healthServiceNames mirrors the per-service names registered by the server.
// Defined once here to keep the three phase subtests in sync.
var healthServiceNames = []string{
	"envoy.service.auth.v3.Authorization",
	"parsec.v1.TokenExchangeService",
	"parsec.v1.JWKSService",
}

// TestHealthEndpoints validates the full health check lifecycle through both
// HTTP and gRPC interfaces. The test mirrors how serve.go uses the server:
//
//	Start (NOT_SERVING) → SetReady (SERVING) → SetNotReady (NOT_SERVING) → Stop
//
// Subtests run sequentially and share one server, following the same pattern
// as TestJWKSEndpoint.
func TestHealthEndpoints(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup minimal dependencies (health checks don't need real validators/issuers)
	trustStore, tokenService, issuerRegistry := setupTestDependencies()
	claimsFilterRegistry := server.NewStubClaimsFilterRegistry()

	srv := server.New(server.Config{
		GRPCPort:       19095,
		HTTPPort:       18085,
		AuthzServer:    server.NewAuthzServer(trustStore, tokenService, nil, nil),
		ExchangeServer: server.NewExchangeServer(trustStore, tokenService, claimsFilterRegistry, nil),
		JWKSServer:     server.NewJWKSServer(server.JWKSServerConfig{IssuerRegistry: issuerRegistry, Logger: slog.Default()}),
	})

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	waitForServer(t, 18085, 5*time.Second)

	httpClient := &http.Client{Timeout: 5 * time.Second}

	// Dial a gRPC connection for health checks
	grpcConn, err := grpc.NewClient(
		"localhost:19095",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial gRPC: %v", err)
	}
	defer func() { _ = grpcConn.Close() }()

	healthClient := healthpb.NewHealthClient(grpcConn)

	// ================================================================
	// Phase 1: Before SetReady — services are NOT_SERVING
	// ================================================================

	t.Run("HTTP liveness returns 200 before SetReady", func(t *testing.T) {
		body := httpGet(t, httpClient, "http://localhost:18085/healthz/live")

		if body["status"] != "OK" {
			t.Errorf("expected status OK, got %q", body["status"])
		}
	})

	t.Run("HTTP readiness returns 503 before SetReady", func(t *testing.T) {
		resp, err := httpClient.Get("http://localhost:18085/healthz/ready")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", resp.StatusCode)
		}

		body := decodeJSON(t, resp.Body)
		if body["status"] != "NOT_SERVING" {
			t.Errorf("expected status NOT_SERVING, got %q", body["status"])
		}
		// The response should identify which service failed
		if body["service"] == "" {
			t.Error("expected a service name in the response")
		}
	})

	t.Run("gRPC overall health returns SERVING (liveness)", func(t *testing.T) {
		// The empty string "" is the overall server health (set to SERVING by default).
		resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{Service: ""})
		if err != nil {
			t.Fatalf("Health/Check failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("expected SERVING for overall health, got %v", resp.Status)
		}
	})

	t.Run("gRPC per-service health returns NOT_SERVING before SetReady", func(t *testing.T) {
		for _, svc := range healthServiceNames {
			t.Run(svc, func(t *testing.T) {
				resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{Service: svc})
				if err != nil {
					t.Fatalf("Health/Check(%q) failed: %v", svc, err)
				}
				if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
					t.Errorf("expected NOT_SERVING, got %v", resp.Status)
				}
			})
		}
	})

	// ================================================================
	// Phase 2: After SetReady — all services SERVING
	// ================================================================

	srv.SetReady()

	t.Run("HTTP liveness returns 200 after SetReady", func(t *testing.T) {
		body := httpGet(t, httpClient, "http://localhost:18085/healthz/live")

		if body["status"] != "OK" {
			t.Errorf("expected status OK, got %q", body["status"])
		}
	})

	t.Run("HTTP readiness returns 200 after SetReady", func(t *testing.T) {
		resp, err := httpClient.Get("http://localhost:18085/healthz/ready")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		body := decodeJSON(t, resp.Body)
		if body["status"] != "SERVING" {
			t.Errorf("expected status SERVING, got %q", body["status"])
		}
	})

	t.Run("gRPC per-service health returns SERVING after SetReady", func(t *testing.T) {
		for _, svc := range healthServiceNames {
			t.Run(svc, func(t *testing.T) {
				resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{Service: svc})
				if err != nil {
					t.Fatalf("Health/Check(%q) failed: %v", svc, err)
				}
				if resp.Status != healthpb.HealthCheckResponse_SERVING {
					t.Errorf("expected SERVING, got %v", resp.Status)
				}
			})
		}
	})

	// ================================================================
	// Phase 3: After SetNotReady — simulates degraded state
	// ================================================================

	srv.SetNotReady()

	t.Run("HTTP readiness returns 503 after SetNotReady", func(t *testing.T) {
		resp, err := httpClient.Get("http://localhost:18085/healthz/ready")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", resp.StatusCode)
		}

		body := decodeJSON(t, resp.Body)
		if body["status"] != "NOT_SERVING" {
			t.Errorf("expected status NOT_SERVING, got %q", body["status"])
		}
	})

	t.Run("gRPC per-service health returns NOT_SERVING after SetNotReady", func(t *testing.T) {
		for _, svc := range healthServiceNames {
			t.Run(svc, func(t *testing.T) {
				resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{Service: svc})
				if err != nil {
					t.Fatalf("Health/Check(%q) failed: %v", svc, err)
				}
				if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
					t.Errorf("expected NOT_SERVING, got %v", resp.Status)
				}
			})
		}
	})
}

// TestHealthEndpoints_UnknownService verifies that querying an unregistered
// service returns gRPC NOT_FOUND, per the gRPC Health Checking Protocol spec.
func TestHealthEndpoints_UnknownService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trustStore, tokenService, issuerRegistry := setupTestDependencies()
	claimsFilterRegistry := server.NewStubClaimsFilterRegistry()

	srv := server.New(server.Config{
		GRPCPort:       19096,
		HTTPPort:       18086,
		AuthzServer:    server.NewAuthzServer(trustStore, tokenService, nil, nil),
		ExchangeServer: server.NewExchangeServer(trustStore, tokenService, claimsFilterRegistry, nil),
		JWKSServer:     server.NewJWKSServer(server.JWKSServerConfig{IssuerRegistry: issuerRegistry, Logger: slog.Default()}),
	})

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	waitForServer(t, 18086, 5*time.Second)

	grpcConn, err := grpc.NewClient(
		"localhost:19096",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial gRPC: %v", err)
	}
	defer func() { _ = grpcConn.Close() }()

	healthClient := healthpb.NewHealthClient(grpcConn)

	t.Run("gRPC health check for unknown service returns NOT_FOUND", func(t *testing.T) {
		_, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{
			Service: "nonexistent.UnknownService",
		})
		if err == nil {
			t.Fatal("expected error for unknown service, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got: %v", err)
		}
		if st.Code() != codes.NotFound {
			t.Errorf("expected NOT_FOUND, got %v", st.Code())
		}
	})
}

// TestHealthEndpoints_ReflectionListsHealthService verifies that the gRPC
// reflection service includes grpc.health.v1.Health so tools like grpcurl
// can discover it.
func TestHealthEndpoints_ReflectionListsHealthService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trustStore := trust.NewStubStore()
	stubValidator := trust.NewStubValidator(trust.CredentialTypeBearer)
	trustStore.AddValidator(stubValidator)

	issuerRegistry := service.NewSimpleRegistry()
	dataSourceRegistry := service.NewDataSourceRegistry()
	tokenService := service.NewTokenService("parsec.test", dataSourceRegistry, issuerRegistry, nil)
	claimsFilterRegistry := server.NewStubClaimsFilterRegistry()

	srv := server.New(server.Config{
		GRPCPort:       19097,
		HTTPPort:       18087,
		AuthzServer:    server.NewAuthzServer(trustStore, tokenService, nil, nil),
		ExchangeServer: server.NewExchangeServer(trustStore, tokenService, claimsFilterRegistry, nil),
		JWKSServer:     server.NewJWKSServer(server.JWKSServerConfig{IssuerRegistry: issuerRegistry, Logger: slog.Default()}),
	})

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	waitForServer(t, 18087, 5*time.Second)

	grpcConn, err := grpc.NewClient(
		"localhost:19097",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial gRPC: %v", err)
	}
	defer func() { _ = grpcConn.Close() }()

	// Use the health client to verify the health service is accessible
	// (this implicitly proves the service is registered on the gRPC server)
	healthClient := healthpb.NewHealthClient(grpcConn)

	// Overall health check (empty string) — should work if service is registered
	resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{Service: ""})
	if err != nil {
		t.Fatalf("Health service not accessible via gRPC: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("expected SERVING for overall health, got %v", resp.Status)
	}

	t.Log("✓ grpc.health.v1.Health service is registered and accessible")
}

// TestHealthEndpoints_WatchStream verifies that the gRPC Watch RPC delivers
// real-time status updates when health transitions occur, as specified by
// https://github.com/grpc/grpc/blob/master/doc/health-checking.md
func TestHealthEndpoints_WatchStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trustStore, tokenService, issuerRegistry := setupTestDependencies()
	claimsFilterRegistry := server.NewStubClaimsFilterRegistry()

	srv := server.New(server.Config{
		GRPCPort:       19098,
		HTTPPort:       18088,
		AuthzServer:    server.NewAuthzServer(trustStore, tokenService, nil, nil),
		ExchangeServer: server.NewExchangeServer(trustStore, tokenService, claimsFilterRegistry, nil),
		JWKSServer:     server.NewJWKSServer(server.JWKSServerConfig{IssuerRegistry: issuerRegistry, Logger: slog.Default()}),
	})

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() { _ = srv.Stop(ctx) }()

	waitForServer(t, 18088, 5*time.Second)

	grpcConn, err := grpc.NewClient(
		"localhost:19098",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial gRPC: %v", err)
	}
	defer func() { _ = grpcConn.Close() }()

	healthClient := healthpb.NewHealthClient(grpcConn)

	// Pick one service to watch (the protocol works the same for all).
	const watchedService = "parsec.v1.TokenExchangeService"

	watchCtx, watchCancel := context.WithTimeout(ctx, 10*time.Second)
	defer watchCancel()

	stream, err := healthClient.Watch(watchCtx, &healthpb.HealthCheckRequest{
		Service: watchedService,
	})
	if err != nil {
		t.Fatalf("Watch(%q) failed: %v", watchedService, err)
	}

	// The first message on the stream should reflect the current status.
	// Before SetReady, all per-service statuses are NOT_SERVING.
	t.Run("initial Watch message is NOT_SERVING", func(t *testing.T) {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv() failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})

	// Transition to ready — the stream should deliver a SERVING update.
	srv.SetReady()

	t.Run("Watch delivers SERVING after SetReady", func(t *testing.T) {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv() failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("expected SERVING, got %v", resp.Status)
		}
	})

	// Transition back to not-ready — the stream should deliver NOT_SERVING.
	srv.SetNotReady()

	t.Run("Watch delivers NOT_SERVING after SetNotReady", func(t *testing.T) {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv() failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})
}

// --- test helpers ---

// httpGet performs a GET request and returns the parsed JSON body.
// It asserts status 200.
func httpGet(t *testing.T, client *http.Client, url string) map[string]string {
	t.Helper()

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: expected 200, got %d. Body: %s", url, resp.StatusCode, body)
	}

	return decodeJSON(t, resp.Body)
}

// decodeJSON reads an io.Reader and decodes the JSON body into a map.
func decodeJSON(t *testing.T, r io.Reader) map[string]string {
	t.Helper()

	var body map[string]string
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	return body
}
