package metrics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/project-kessel/parsec/internal/datasource"
	"github.com/project-kessel/parsec/internal/service"
)

func testProvider(t *testing.T) *Provider {
	t.Helper()
	reg := prometheus.NewRegistry()
	p, err := New(WithRegistry(reg))
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	return p
}

func scrape(t *testing.T, p *Provider) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	p.Handler().ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Body)
	return string(body)
}

func TestNewObserver_SatisfiesObserverInterface(t *testing.T) {
	p := testProvider(t)
	obs, err := NewObserver(p)
	require.NoError(t, err)
	require.NotNil(t, obs)
}

func TestTokenIssuanceMetrics_Success(t *testing.T) {
	p := testProvider(t)
	obs, err := NewObserver(p)
	require.NoError(t, err)

	ctx := context.Background()
	_, probe := obs.TokenIssuanceStarted(ctx, nil, nil, "test", []service.TokenType{"jwt"})
	probe.End()

	body := scrape(t, p)
	assert.Contains(t, body, "parsec_token_issuance_total")
	assert.Contains(t, body, `status="success"`)
	assert.Contains(t, body, "parsec_token_issuance_duration_seconds")
}

func TestTokenIssuanceMetrics_Error(t *testing.T) {
	p := testProvider(t)
	obs, err := NewObserver(p)
	require.NoError(t, err)

	ctx := context.Background()
	_, probe := obs.TokenIssuanceStarted(ctx, nil, nil, "test", []service.TokenType{"jwt"})
	probe.TokenTypeIssuanceFailed("jwt", errors.New("sign error"))
	probe.End()

	body := scrape(t, p)
	assert.Contains(t, body, `status="error"`)
}

func TestTokenExchangeMetrics(t *testing.T) {
	p := testProvider(t)
	obs, err := NewObserver(p)
	require.NoError(t, err)

	ctx := context.Background()
	_, probe := obs.TokenExchangeStarted(ctx, "urn:ietf:params:oauth:grant-type:token-exchange", "jwt", "aud", "scope")
	probe.End()

	body := scrape(t, p)
	assert.Contains(t, body, "parsec_token_exchange_total")
	assert.Contains(t, body, "parsec_token_exchange_duration_seconds")
}

func TestAuthzCheckMetrics(t *testing.T) {
	p := testProvider(t)
	obs, err := NewObserver(p)
	require.NoError(t, err)

	ctx := context.Background()
	_, probe := obs.AuthzCheckStarted(ctx)
	probe.ActorValidationFailed(errors.New("bad actor"))
	probe.End()

	body := scrape(t, p)
	assert.Contains(t, body, "parsec_authz_check_total")
	assert.Contains(t, body, `status="error"`)
}

func TestCacheFetchStarted(t *testing.T) {
	tests := []struct {
		name           string
		dataSourceName string
		action         func(probe datasource.CacheFetchProbe)
		wantResult     string
		wantStatus     string
	}{
		{
			name:           "hit",
			dataSourceName: "inventory",
			action:         func(p datasource.CacheFetchProbe) { p.CacheHit() },
			wantResult:     `result="hit"`,
			wantStatus:     `status="success"`,
		},
		{
			name:           "miss",
			dataSourceName: "inventory",
			action:         func(p datasource.CacheFetchProbe) { p.CacheMiss() },
			wantResult:     `result="miss"`,
			wantStatus:     `status="success"`,
		},
		{
			name:           "expired",
			dataSourceName: "inventory",
			action:         func(p datasource.CacheFetchProbe) { p.CacheExpired() },
			wantResult:     `result="expired"`,
			wantStatus:     `status="success"`,
		},
		{
			name:           "error",
			dataSourceName: "inventory",
			action:         func(p datasource.CacheFetchProbe) { p.FetchFailed(errors.New("timeout")) },
			wantResult:     `result="error"`,
			wantStatus:     `status="error"`,
		},
		{
			name:           "unknown when no outcome is signaled",
			dataSourceName: "inventory",
			action:         func(datasource.CacheFetchProbe) {},
			wantResult:     `result="unknown"`,
			wantStatus:     `status="success"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testProvider(t)
			obs, err := NewObserver(p)
			require.NoError(t, err)

			_, probe := obs.CacheFetchStarted(context.Background(), tt.dataSourceName)
			tt.action(probe)
			probe.End()

			body := scrape(t, p)
			assert.Contains(t, body, "parsec_datasource_cache_fetch_total")
			assert.Contains(t, body, "parsec_datasource_cache_fetch_duration_seconds")
			assert.Contains(t, body, fmt.Sprintf(`datasource="%s"`, tt.dataSourceName))
			assert.Contains(t, body, tt.wantResult)
			assert.Contains(t, body, tt.wantStatus)
		})
	}
}

func TestTrustValidationMetrics(t *testing.T) {
	p := testProvider(t)
	obs, err := NewObserver(p)
	require.NoError(t, err)

	ctx := context.Background()
	_, probe := obs.ValidationStarted(ctx)
	probe.End()

	body := scrape(t, p)
	assert.Contains(t, body, "parsec_trust_validation_total")
	assert.Contains(t, body, `status="success"`)
}

func TestKeyRotationMetrics(t *testing.T) {
	p := testProvider(t)
	obs, err := NewObserver(p)
	require.NoError(t, err)

	ctx := context.Background()
	_, probe := obs.RotationCheckStarted(ctx)
	probe.RotationCheckFailed(errors.New("timeout"))
	probe.End()

	body := scrape(t, p)
	assert.Contains(t, body, "parsec_keys_rotation_total")
	assert.Contains(t, body, `status="error"`)
}
