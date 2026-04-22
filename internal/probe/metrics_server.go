package probe

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/server"
)

// metricsServerObserver implements server.ServerObserver using OTel metrics.
// Phase 1: skeleton with NoOp embedding. Phase 2 replaces with real instruments.
type metricsServerObserver struct {
	server.NoOpServerObserver
	meter metric.Meter
}

// NewMetricsServerObserver creates a metrics-based server observer.
func NewMetricsServerObserver(meter metric.Meter) server.ServerObserver {
	return &metricsServerObserver{meter: meter}
}

var _ server.ServerObserver = (*metricsServerObserver)(nil)
