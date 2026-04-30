package metrics

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/trust"
)

type trustObserver struct {
	trust.NoOpTrustObserver

	validationTotal    metric.Int64Counter
	validationDuration metric.Float64Histogram
}

func newTrustObserver(m metric.Meter) (*trustObserver, error) {
	vt, err := m.Int64Counter("parsec.trust.validation.total",
		metric.WithDescription("Total trust validation operations"),
	)
	if err != nil {
		return nil, err
	}
	vd, err := m.Float64Histogram("parsec.trust.validation.duration",
		metric.WithDescription("Trust validation duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &trustObserver{
		validationTotal:    vt,
		validationDuration: vd,
	}, nil
}

func (o *trustObserver) ValidationStarted(ctx context.Context) (context.Context, trust.ValidationProbe) {
	return ctx, &validationProbe{metricProbe: metricProbe{
		ctx:       ctx,
		counter:   o.validationTotal,
		histogram: o.validationDuration,
		startTime: time.Now(),
	}}
}

type validationProbe struct {
	trust.NoOpValidationProbe
	metricProbe
}

func (p *validationProbe) AllValidatorsFailed(_ trust.CredentialType, _ int, _ error) { p.markFailed() }
func (p *validationProbe) End()                                                       { p.record() }

var (
	_ trust.TrustObserver   = (*trustObserver)(nil)
	_ trust.ValidationProbe = (*validationProbe)(nil)
)
