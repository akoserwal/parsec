package metrics

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Provider manages the OTel MeterProvider lifecycle and Prometheus HTTP handler.
type Provider struct {
	meterProvider *sdkmetric.MeterProvider
}

// NewProvider creates a MeterProvider backed by a Prometheus exporter.
// The exporter automatically registers with the default Prometheus registry,
// making metrics available via Handler().
func NewProvider() (*Provider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	return &Provider{meterProvider: mp}, nil
}

// Meter returns a named Meter for creating instruments.
func (p *Provider) Meter(name string) metric.Meter {
	return p.meterProvider.Meter(name)
}

// Handler returns an http.Handler that serves Prometheus metrics.
func (p *Provider) Handler() http.Handler {
	return promhttp.Handler()
}

// Shutdown flushes pending metrics and releases resources.
func (p *Provider) Shutdown(ctx context.Context) error {
	return p.meterProvider.Shutdown(ctx)
}
