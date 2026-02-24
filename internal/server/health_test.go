package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

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

// TestHealthReadinessIdentifiesFailingService verifies that when a single
// service transitions to NOT_SERVING, the HTTP readiness endpoint returns
// 503 and names that specific service in the response body.
func TestHealthReadinessIdentifiesFailingService(t *testing.T) {
	env := startTestServer(t, stubServerConfig())
	env.Srv.SetReady()

	for _, svc := range healthServices {
		t.Run(svc, func(t *testing.T) {
			env.Srv.healthServer.SetServingStatus(svc, healthpb.HealthCheckResponse_NOT_SERVING)
			defer env.Srv.healthServer.SetServingStatus(svc, healthpb.HealthCheckResponse_SERVING)

			resp, err := env.HTTPClient.Get(env.HTTPBaseURL + "/healthz/ready")
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d", resp.StatusCode)
			}

			body := decodeJSONBody(t, resp.Body)
			if body["service"] != svc {
				t.Errorf("expected failing service %q, got %q", svc, body["service"])
			}
		})
	}
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
