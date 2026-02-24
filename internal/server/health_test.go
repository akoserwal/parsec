package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

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

		resp, err := srv.healthServer.Check(nil, &healthpb.HealthCheckRequest{
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

		resp, err := srv.healthServer.Check(nil, &healthpb.HealthCheckRequest{
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

		resp, err := srv.healthServer.Check(nil, &healthpb.HealthCheckRequest{
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

		resp, err := srv.healthServer.Check(nil, &healthpb.HealthCheckRequest{
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

// --- test helpers (bottom of file, per codebase convention) ---

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

// checkLiveness calls handleLiveness and returns the recorded response.
func checkLiveness(t *testing.T, srv *Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/healthz/live", nil)
	rec := httptest.NewRecorder()
	srv.handleLiveness(rec, req)
	return rec
}

// checkReadiness calls handleReadiness and returns the recorded response.
func checkReadiness(t *testing.T, srv *Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	srv.handleReadiness(rec, req)
	return rec
}

// assertHealthResponse checks the HTTP status code and the "status" field in
// the JSON body. It returns the decoded body for further assertions (e.g.
// checking the "service" field).
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
