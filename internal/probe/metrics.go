package probe

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/service"
)

// metricsServiceObserver implements service.ServiceObserver using OTel metrics.
// Phase 1: skeleton with NoOp embedding. Phase 2 replaces with real instruments.
type metricsServiceObserver struct {
	service.NoOpServiceObserver
	meter metric.Meter
}

// NewMetricsServiceObserver creates a metrics-based service observer.
func NewMetricsServiceObserver(meter metric.Meter) service.ServiceObserver {
	return &metricsServiceObserver{meter: meter}
}

var _ service.ServiceObserver = (*metricsServiceObserver)(nil)
