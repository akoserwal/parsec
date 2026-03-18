package probe

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/project-kessel/parsec/internal/service"
)

// testMeter returns a Meter backed by a ManualReader for assertions.
func testMeter(t *testing.T) (*sdkmetric.ManualReader, *sdkmetric.MeterProvider) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	return reader, mp
}

// collectMetrics forces a metric collection and returns the data.
func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	return rm
}

// findMetric searches for a metric by name across all scopes.
func findMetric(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

// sumCounterValue returns the total value across all data points of a Sum metric.
func sumCounterValue(m *metricdata.Metrics) int64 {
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		return 0
	}
	var total int64
	for _, dp := range sum.DataPoints {
		total += dp.Value
	}
	return total
}

// countHistogramPoints returns the number of recorded data points in a histogram.
func countHistogramPoints(m *metricdata.Metrics) uint64 {
	hist, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		return 0
	}
	var total uint64
	for _, dp := range hist.DataPoints {
		total += dp.Count
	}
	return total
}

// counterValueWithAttr returns the value of the data point matching the given key/value pair.
func counterValueWithAttr(m *metricdata.Metrics, key, value string) int64 {
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		return 0
	}
	for _, dp := range sum.DataPoints {
		for _, attr := range dp.Attributes.ToSlice() {
			if string(attr.Key) == key && attr.Value.AsString() == value {
				return dp.Value
			}
		}
	}
	return 0
}

// --- Token Issuance ---

func TestMetricsObserver_TokenIssuance_Success(t *testing.T) {
	reader, mp := testMeter(t)
	obs, err := NewMetricsObserver(mp.Meter("test"))
	require.NoError(t, err)

	ctx, probe := obs.TokenIssuanceStarted(context.Background(), nil, nil, "openid", []service.TokenType{"txn_token"})
	assert.NotNil(t, ctx)

	probe.TokenTypeIssuanceStarted("txn_token")
	probe.TokenTypeIssuanceSucceeded("txn_token", &service.Token{})
	probe.End()

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "parsec_token_issuance_total")
	require.NotNil(t, total, "expected parsec_token_issuance_total metric")
	assert.Equal(t, int64(1), counterValueWithAttr(total, "status", "success"))

	dur := findMetric(rm, "parsec_token_issuance_duration_seconds")
	require.NotNil(t, dur, "expected parsec_token_issuance_duration_seconds metric")
	assert.Equal(t, uint64(1), countHistogramPoints(dur))
}

func TestMetricsObserver_TokenIssuance_Failure(t *testing.T) {
	reader, mp := testMeter(t)
	obs, err := NewMetricsObserver(mp.Meter("test"))
	require.NoError(t, err)

	_, probe := obs.TokenIssuanceStarted(context.Background(), nil, nil, "", nil)
	probe.TokenTypeIssuanceFailed("txn_token", errors.New("signing error"))
	probe.End()

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "parsec_token_issuance_total")
	require.NotNil(t, total)
	assert.Equal(t, int64(1), counterValueWithAttr(total, "status", "failure"))
}

func TestMetricsObserver_TokenIssuance_IssuerNotFound(t *testing.T) {
	reader, mp := testMeter(t)
	obs, err := NewMetricsObserver(mp.Meter("test"))
	require.NoError(t, err)

	_, probe := obs.TokenIssuanceStarted(context.Background(), nil, nil, "", nil)
	probe.IssuerNotFound("unknown_type", errors.New("no issuer"))
	probe.End()

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "parsec_token_issuance_total")
	require.NotNil(t, total)
	assert.Equal(t, int64(1), counterValueWithAttr(total, "status", "issuer_not_found"))
}

// --- Token Exchange ---

func TestMetricsObserver_TokenExchange_Success(t *testing.T) {
	reader, mp := testMeter(t)
	obs, err := NewMetricsObserver(mp.Meter("test"))
	require.NoError(t, err)

	ctx, probe := obs.TokenExchangeStarted(context.Background(), "urn:ietf:params:oauth:grant-type:token-exchange", "urn:ietf:params:oauth:token-type:txn_token", "aud", "openid")
	assert.NotNil(t, ctx)

	probe.ActorValidationSucceeded(nil)
	probe.SubjectTokenValidationSucceeded(nil)
	probe.End()

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "parsec_token_exchange_total")
	require.NotNil(t, total)
	assert.Equal(t, int64(1), counterValueWithAttr(total, "status", "success"))

	dur := findMetric(rm, "parsec_token_exchange_duration_seconds")
	require.NotNil(t, dur)
	assert.Equal(t, uint64(1), countHistogramPoints(dur))

	failures := findMetric(rm, "parsec_token_exchange_validation_failures_total")
	if failures != nil {
		assert.Equal(t, int64(0), sumCounterValue(failures))
	}
}

func TestMetricsObserver_TokenExchange_ValidationFailures(t *testing.T) {
	reader, mp := testMeter(t)
	obs, err := NewMetricsObserver(mp.Meter("test"))
	require.NoError(t, err)

	_, probe := obs.TokenExchangeStarted(context.Background(), "", "", "", "")
	probe.ActorValidationFailed(errors.New("bad actor"))
	probe.SubjectTokenValidationFailed(errors.New("bad subject"))
	probe.RequestContextParseFailed(errors.New("bad context"))
	probe.End()

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "parsec_token_exchange_total")
	require.NotNil(t, total)
	assert.Equal(t, int64(1), counterValueWithAttr(total, "status", "failure"))

	failures := findMetric(rm, "parsec_token_exchange_validation_failures_total")
	require.NotNil(t, failures)
	assert.Equal(t, int64(1), counterValueWithAttr(failures, "stage", "actor"))
	assert.Equal(t, int64(1), counterValueWithAttr(failures, "stage", "subject"))
	assert.Equal(t, int64(1), counterValueWithAttr(failures, "stage", "request_context"))
}

// --- Authz Check ---

func TestMetricsObserver_AuthzCheck_Success(t *testing.T) {
	reader, mp := testMeter(t)
	obs, err := NewMetricsObserver(mp.Meter("test"))
	require.NoError(t, err)

	ctx, probe := obs.AuthzCheckStarted(context.Background())
	assert.NotNil(t, ctx)

	probe.ActorValidationSucceeded(nil)
	probe.SubjectValidationSucceeded(nil)
	probe.End()

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "parsec_authz_check_total")
	require.NotNil(t, total)
	assert.Equal(t, int64(1), counterValueWithAttr(total, "status", "success"))

	dur := findMetric(rm, "parsec_authz_check_duration_seconds")
	require.NotNil(t, dur)
	assert.Equal(t, uint64(1), countHistogramPoints(dur))
}

func TestMetricsObserver_AuthzCheck_ValidationFailures(t *testing.T) {
	reader, mp := testMeter(t)
	obs, err := NewMetricsObserver(mp.Meter("test"))
	require.NoError(t, err)

	_, probe := obs.AuthzCheckStarted(context.Background())
	probe.ActorValidationFailed(errors.New("bad actor"))
	probe.SubjectValidationFailed(errors.New("bad subject"))
	probe.SubjectCredentialExtractionFailed(errors.New("no creds"))
	probe.End()

	rm := collectMetrics(t, reader)

	total := findMetric(rm, "parsec_authz_check_total")
	require.NotNil(t, total)
	assert.Equal(t, int64(1), counterValueWithAttr(total, "status", "failure"))

	failures := findMetric(rm, "parsec_authz_check_validation_failures_total")
	require.NotNil(t, failures)
	assert.Equal(t, int64(1), counterValueWithAttr(failures, "stage", "actor"))
	assert.Equal(t, int64(1), counterValueWithAttr(failures, "stage", "subject"))
	assert.Equal(t, int64(1), counterValueWithAttr(failures, "stage", "subject_extraction"))
}

// --- Observer creation ---

func TestNewMetricsObserver_ImplementsApplicationObserver(t *testing.T) {
	_, mp := testMeter(t)
	obs, err := NewMetricsObserver(mp.Meter("test"))
	require.NoError(t, err)

	var _ service.ApplicationObserver = obs
}
