package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_CreatesProvider(t *testing.T) {
	p, err := New()
	require.NoError(t, err)
	require.NotNil(t, p)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	assert.NotNil(t, p.Handler())
	assert.NotNil(t, p.Meter("test"))
}

func TestNew_WithCustomRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, err := New(WithRegistry(reg))
	require.NoError(t, err)
	require.NotNil(t, p)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
}

func TestProvider_Handler_ServesMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, err := New(WithRegistry(reg))
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	p.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestProvider_Shutdown(t *testing.T) {
	p, err := New()
	require.NoError(t, err)

	err = p.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestProvider_MetricsEndpoint_ContainsOTelMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	p, err := New(WithRegistry(reg))
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	m := p.Meter("test")
	counter, err := m.Int64Counter("test_counter")
	require.NoError(t, err)
	counter.Add(context.Background(), 42)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	p.Handler().ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	assert.Contains(t, string(body), "test_counter")
}
