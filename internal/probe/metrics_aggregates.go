package probe

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/observer"
)

// NewMetricsObserver builds a central observer.Observer backed by OTel metrics.
// Domain-scoped meters group metrics by component in OTLP export.
func NewMetricsObserver(mp metric.MeterProvider) observer.Observer {
	return observer.Compose(
		NewMetricsServiceObserver(mp.Meter("parsec.service")),
		NewMetricsDataSourceObserver(mp.Meter("parsec.datasource")),
		NewMetricsKeysObserver(mp.Meter("parsec.keys")),
		NewMetricsTrustObserver(mp.Meter("parsec.trust")),
		NewMetricsServerObserver(mp.Meter("parsec.server")),
	)
}
