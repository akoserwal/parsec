package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/project-kessel/parsec/internal/server"
)

func TestGRPCStatsHandler_Nil_ReturnsNonNil(t *testing.T) {
	h := server.GRPCStatsHandler(nil)
	require.NotNil(t, h)
}

func TestGRPCStatsHandler_WithProvider_ReturnsNonNil(t *testing.T) {
	mp := sdkmetric.NewMeterProvider()
	defer func() { _ = mp.Shutdown(context.Background()) }()

	h := server.GRPCStatsHandler(mp)
	require.NotNil(t, h)
}

func TestMetricsHTTPMiddleware_Nil_ReturnsOriginal(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := server.MetricsHTTPMiddleware(nil, inner)
	assert.NotNil(t, wrapped)

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsHTTPMiddleware_WithProvider_ReturnsWrapped(t *testing.T) {
	mp := sdkmetric.NewMeterProvider()
	defer func() { _ = mp.Shutdown(context.Background()) }()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := server.MetricsHTTPMiddleware(mp, inner)
	require.NotNil(t, wrapped)

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest("GET", "/test", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}
