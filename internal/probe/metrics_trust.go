package probe

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/trust"
)

// metricsTrustObserver implements trust.TrustObserver using OTel metrics.
// Phase 1: skeleton with NoOp embedding. Phase 2 replaces with real instruments.
type metricsTrustObserver struct {
	trust.NoOpTrustObserver
	meter metric.Meter
}

// NewMetricsTrustObserver creates a metrics-based trust observer.
func NewMetricsTrustObserver(meter metric.Meter) trust.TrustObserver {
	return &metricsTrustObserver{meter: meter}
}

var _ trust.TrustObserver = (*metricsTrustObserver)(nil)
