package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// Unit tests — exercise handler functions directly, no network
// ---------------------------------------------------------------------------

func TestHandleLiveness(t *testing.T) {
	srv := newHealthTestServer()

	t.Run("always returns 200 OK", func(t *testing.T) {
		rec := checkLiveness(t, srv)
		assertHealthResponse(t, rec, http.StatusOK, "OK")
	})
}

func TestHandleReadiness(t *testing.T) {
	t.Run("returns 200 when all services are SERVING", func(t *testing.T) {
		srv := newHealthTestServer()
		srv.SetReady()

		rec := checkReadiness(t, srv)
		assertHealthResponse(t, rec, http.StatusOK, "SERVING")
	})

	t.Run("returns 503 when no services are SERVING", func(t *testing.T) {
		srv := newHealthTestServer() // all start as NOT_SERVING

		rec := checkReadiness(t, srv)
		body := assertHealthResponse(t, rec, http.StatusServiceUnavailable, "NOT_SERVING")

		// The first service in the list should be reported.
		if body["service"] != healthServices[0] {
			t.Errorf("expected service %q, got %q", healthServices[0], body["service"])
		}
	})

	t.Run("returns 503 identifying the specific failing service", func(t *testing.T) {
		for _, failing := range healthServices {
			t.Run(failing, func(t *testing.T) {
				srv := newHealthTestServer()
				srv.SetReady() // all SERVING

				// Break just this one service.
				srv.healthServer.SetServingStatus(failing, healthpb.HealthCheckResponse_NOT_SERVING)

				rec := checkReadiness(t, srv)
				body := assertHealthResponse(t, rec, http.StatusServiceUnavailable, "NOT_SERVING")

				if body["service"] != failing {
					t.Errorf("expected failing service %q, got %q", failing, body["service"])
				}
			})
		}
	})

	t.Run("returns 503 after health server shutdown", func(t *testing.T) {
		srv := newHealthTestServer()
		srv.SetReady()

		// Shutdown marks everything NOT_SERVING and ignores future SetServingStatus calls.
		srv.healthServer.Shutdown()

		rec := checkReadiness(t, srv)
		assertHealthResponse(t, rec, http.StatusServiceUnavailable, "NOT_SERVING")
	})
}

func TestReadinessServiceKey(t *testing.T) {
	t.Run("starts as NOT_SERVING", func(t *testing.T) {
		srv := newHealthTestServer()

		resp, err := srv.healthServer.Check(context.Background(), &healthpb.HealthCheckRequest{
			Service: healthReadinessService,
		})
		if err != nil {
			t.Fatalf("Check(%q) failed: %v", healthReadinessService, err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})

	t.Run("transitions to SERVING after SetReady", func(t *testing.T) {
		srv := newHealthTestServer()
		srv.SetReady()

		resp, err := srv.healthServer.Check(context.Background(), &healthpb.HealthCheckRequest{
			Service: healthReadinessService,
		})
		if err != nil {
			t.Fatalf("Check(%q) failed: %v", healthReadinessService, err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("expected SERVING, got %v", resp.Status)
		}
	})

	t.Run("transitions to NOT_SERVING after SetNotReady", func(t *testing.T) {
		srv := newHealthTestServer()
		srv.SetReady()
		srv.SetNotReady()

		resp, err := srv.healthServer.Check(context.Background(), &healthpb.HealthCheckRequest{
			Service: healthReadinessService,
		})
		if err != nil {
			t.Fatalf("Check(%q) failed: %v", healthReadinessService, err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})

	t.Run("transitions to NOT_SERVING after Shutdown", func(t *testing.T) {
		srv := newHealthTestServer()
		srv.SetReady()
		srv.healthServer.Shutdown()

		resp, err := srv.healthServer.Check(context.Background(), &healthpb.HealthCheckRequest{
			Service: healthReadinessService,
		})
		if err != nil {
			t.Fatalf("Check(%q) failed: %v", healthReadinessService, err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})
}

// ---------------------------------------------------------------------------
// Server tests — exercise the full gRPC + HTTP stack via startTestServer
// ---------------------------------------------------------------------------

// TestHealthLifecycle validates the full health check lifecycle through both
// HTTP and gRPC interfaces:
//
//	Start (NOT_SERVING) → SetReady (SERVING) → SetNotReady (NOT_SERVING) → Stop
func TestHealthLifecycle(t *testing.T) {
	env := startTestServer(t, stubServerConfig())

	// Phase 1: Before SetReady — services are NOT_SERVING

	t.Run("HTTP liveness returns 200 before SetReady", func(t *testing.T) {
		body := httpGetJSON(t, env.HTTPClient, env.HTTPBaseURL+"/healthz/live")

		if body["status"] != "OK" {
			t.Errorf("expected status OK, got %q", body["status"])
		}
	})

	t.Run("HTTP readiness returns 503 before SetReady", func(t *testing.T) {
		resp, err := env.HTTPClient.Get(env.HTTPBaseURL + "/healthz/ready")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", resp.StatusCode)
		}

		body := decodeJSONBody(t, resp.Body)
		if body["status"] != "NOT_SERVING" {
			t.Errorf("expected status NOT_SERVING, got %q", body["status"])
		}
		if body["service"] == "" {
			t.Error("expected a service name in the response")
		}
	})

	t.Run("gRPC overall health returns SERVING (liveness)", func(t *testing.T) {
		resp, err := env.HealthClient.Check(env.Ctx, &healthpb.HealthCheckRequest{Service: ""})
		if err != nil {
			t.Fatalf("Health/Check failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("expected SERVING for overall health, got %v", resp.Status)
		}
	})

	t.Run("gRPC per-service health returns NOT_SERVING before SetReady", func(t *testing.T) {
		for _, svc := range healthServices {
			t.Run(svc, func(t *testing.T) {
				resp, err := env.HealthClient.Check(env.Ctx, &healthpb.HealthCheckRequest{Service: svc})
				if err != nil {
					t.Fatalf("Health/Check(%q) failed: %v", svc, err)
				}
				if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
					t.Errorf("expected NOT_SERVING, got %v", resp.Status)
				}
			})
		}
	})

	t.Run("gRPC readiness key returns NOT_SERVING before SetReady", func(t *testing.T) {
		resp, err := env.HealthClient.Check(env.Ctx, &healthpb.HealthCheckRequest{Service: "readiness"})
		if err != nil {
			t.Fatalf("Health/Check(readiness) failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})

	// Phase 2: After SetReady — all services SERVING

	env.Srv.SetReady()

	t.Run("HTTP readiness returns 200 after SetReady", func(t *testing.T) {
		resp, err := env.HTTPClient.Get(env.HTTPBaseURL + "/healthz/ready")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		body := decodeJSONBody(t, resp.Body)
		if body["status"] != "SERVING" {
			t.Errorf("expected status SERVING, got %q", body["status"])
		}
	})

	t.Run("gRPC per-service health returns SERVING after SetReady", func(t *testing.T) {
		for _, svc := range healthServices {
			t.Run(svc, func(t *testing.T) {
				resp, err := env.HealthClient.Check(env.Ctx, &healthpb.HealthCheckRequest{Service: svc})
				if err != nil {
					t.Fatalf("Health/Check(%q) failed: %v", svc, err)
				}
				if resp.Status != healthpb.HealthCheckResponse_SERVING {
					t.Errorf("expected SERVING, got %v", resp.Status)
				}
			})
		}
	})

	t.Run("gRPC readiness key returns SERVING after SetReady", func(t *testing.T) {
		resp, err := env.HealthClient.Check(env.Ctx, &healthpb.HealthCheckRequest{Service: "readiness"})
		if err != nil {
			t.Fatalf("Health/Check(readiness) failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("expected SERVING, got %v", resp.Status)
		}
	})

	// Phase 3: After SetNotReady — simulates degraded state

	env.Srv.SetNotReady()

	t.Run("HTTP readiness returns 503 after SetNotReady", func(t *testing.T) {
		resp, err := env.HTTPClient.Get(env.HTTPBaseURL + "/healthz/ready")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", resp.StatusCode)
		}

		body := decodeJSONBody(t, resp.Body)
		if body["status"] != "NOT_SERVING" {
			t.Errorf("expected status NOT_SERVING, got %q", body["status"])
		}
	})

	t.Run("gRPC per-service health returns NOT_SERVING after SetNotReady", func(t *testing.T) {
		for _, svc := range healthServices {
			t.Run(svc, func(t *testing.T) {
				resp, err := env.HealthClient.Check(env.Ctx, &healthpb.HealthCheckRequest{Service: svc})
				if err != nil {
					t.Fatalf("Health/Check(%q) failed: %v", svc, err)
				}
				if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
					t.Errorf("expected NOT_SERVING, got %v", resp.Status)
				}
			})
		}
	})

	t.Run("gRPC readiness key returns NOT_SERVING after SetNotReady", func(t *testing.T) {
		resp, err := env.HealthClient.Check(env.Ctx, &healthpb.HealthCheckRequest{Service: "readiness"})
		if err != nil {
			t.Fatalf("Health/Check(readiness) failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})
}

// TestHealthUnknownService verifies that querying an unregistered service
// returns gRPC NOT_FOUND, per the gRPC Health Checking Protocol spec.
func TestHealthUnknownService(t *testing.T) {
	env := startTestServer(t, stubServerConfig())

	_, err := env.HealthClient.Check(env.Ctx, &healthpb.HealthCheckRequest{
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
}

// TestHealthWatchStream verifies that the gRPC Watch RPC delivers real-time
// status updates when health transitions occur, as specified by
// https://github.com/grpc/grpc/blob/master/doc/health-checking.md
func TestHealthWatchStream(t *testing.T) {
	env := startTestServer(t, stubServerConfig())

	const watchedService = "parsec.v1.TokenExchangeService"

	watchCtx, watchCancel := context.WithTimeout(env.Ctx, 10*time.Second)
	defer watchCancel()

	stream, err := env.HealthClient.Watch(watchCtx, &healthpb.HealthCheckRequest{
		Service: watchedService,
	})
	if err != nil {
		t.Fatalf("Watch(%q) failed: %v", watchedService, err)
	}

	t.Run("initial Watch message is NOT_SERVING", func(t *testing.T) {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv() failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})

	env.Srv.SetReady()

	t.Run("Watch delivers SERVING after SetReady", func(t *testing.T) {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv() failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("expected SERVING, got %v", resp.Status)
		}
	})

	env.Srv.SetNotReady()

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

// TestHealthWatchReadinessKey verifies that the aggregate "readiness" key
// delivers Watch stream updates. This is the key a Kubernetes gRPC readiness
// probe would watch.
func TestHealthWatchReadinessKey(t *testing.T) {
	env := startTestServer(t, stubServerConfig())

	watchCtx, watchCancel := context.WithTimeout(env.Ctx, 10*time.Second)
	defer watchCancel()

	stream, err := env.HealthClient.Watch(watchCtx, &healthpb.HealthCheckRequest{
		Service: "readiness",
	})
	if err != nil {
		t.Fatalf("Watch(readiness) failed: %v", err)
	}

	t.Run("initial Watch message is NOT_SERVING", func(t *testing.T) {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv() failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
			t.Errorf("expected NOT_SERVING, got %v", resp.Status)
		}
	})

	env.Srv.SetReady()

	t.Run("Watch delivers SERVING after SetReady", func(t *testing.T) {
		resp, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv() failed: %v", err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			t.Errorf("expected SERVING, got %v", resp.Status)
		}
	})

	env.Srv.SetNotReady()

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

// --- unit test helpers (handler-level, no network) ---

// newHealthTestServer creates a minimal Server with only the health server
// initialized — enough to exercise handleLiveness, handleReadiness, and
// the readiness service key.
func newHealthTestServer() *Server {
	hs := health.NewServer()
	// Mirror what Start() does: register each service and the aggregate
	// readiness key as NOT_SERVING initially.
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	hs.SetServingStatus(healthReadinessService, healthpb.HealthCheckResponse_NOT_SERVING)
	for _, svc := range healthServices {
		hs.SetServingStatus(svc, healthpb.HealthCheckResponse_NOT_SERVING)
	}
	return &Server{healthServer: hs}
}

func checkLiveness(t *testing.T, srv *Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/healthz/live", nil)
	rec := httptest.NewRecorder()
	srv.handleLiveness(rec, req)
	return rec
}

func checkReadiness(t *testing.T, srv *Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	srv.handleReadiness(rec, req)
	return rec
}

func assertHealthResponse(t *testing.T, rec *httptest.ResponseRecorder, wantCode int, wantStatus string) map[string]string {
	t.Helper()

	if rec.Code != wantCode {
		t.Fatalf("expected HTTP %d, got %d", wantCode, rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}

	if body["status"] != wantStatus {
		t.Errorf("expected status %q, got %q", wantStatus, body["status"])
	}

	return body
}
