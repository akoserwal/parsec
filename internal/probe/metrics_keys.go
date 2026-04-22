package probe

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/keys"
)

// metricsKeysObserver implements keys.KeysObserver using OTel metrics.
// Phase 1: skeleton with NoOp embedding. Phase 2 replaces with real instruments.
type metricsKeysObserver struct {
	keys.NoOpKeysObserver
	meter metric.Meter
}

// NewMetricsKeysObserver creates a metrics-based keys observer.
func NewMetricsKeysObserver(meter metric.Meter) keys.KeysObserver {
	return &metricsKeysObserver{meter: meter}
}

var _ keys.KeysObserver = (*metricsKeysObserver)(nil)
