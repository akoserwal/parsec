package probe

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/datasource"
)

// metricsDataSourceObserver implements datasource.DataSourceObserver using OTel metrics.
// Phase 1: skeleton with NoOp embedding. Phase 2 replaces with real instruments.
type metricsDataSourceObserver struct {
	datasource.NoOpDataSourceObserver
	meter metric.Meter
}

// NewMetricsDataSourceObserver creates a metrics-based datasource observer.
func NewMetricsDataSourceObserver(meter metric.Meter) datasource.DataSourceObserver {
	return &metricsDataSourceObserver{meter: meter}
}

var _ datasource.DataSourceObserver = (*metricsDataSourceObserver)(nil)
