package probe_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/project-kessel/parsec/internal/observer"
	"github.com/project-kessel/parsec/internal/probe"
)

var _ observer.Observer = probe.NewMetricsObserver(noop.NewMeterProvider())

func TestNewMetricsObserver_ReturnsValidObserver(t *testing.T) {
	obs := probe.NewMetricsObserver(noop.NewMeterProvider())
	require.NotNil(t, obs)
}

func TestNewMetricsObserver_TokenIssuanceStarted_NoPanic(t *testing.T) {
	obs := probe.NewMetricsObserver(noop.NewMeterProvider())

	ctx, p := obs.TokenIssuanceStarted(context.Background(), nil, nil, "test", nil)
	assert.NotNil(t, ctx)
	assert.NotNil(t, p)
}

func TestNewMetricsObserver_CacheFetchStarted_NoPanic(t *testing.T) {
	obs := probe.NewMetricsObserver(noop.NewMeterProvider())

	ctx, p := obs.CacheFetchStarted(context.Background(), "test-ds")
	assert.NotNil(t, ctx)
	assert.NotNil(t, p)
}
