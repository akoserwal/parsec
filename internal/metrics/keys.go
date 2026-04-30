package metrics

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/keys"
)

type keysObserver struct {
	keys.NoOpKeysObserver

	rotationTotal    metric.Int64Counter
	rotationDuration metric.Float64Histogram
}

func newKeysObserver(m metric.Meter) (*keysObserver, error) {
	rt, err := m.Int64Counter("parsec.keys.rotation.total",
		metric.WithDescription("Total key rotation operations"),
	)
	if err != nil {
		return nil, err
	}
	rd, err := m.Float64Histogram("parsec.keys.rotation.duration",
		metric.WithDescription("Key rotation duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &keysObserver{
		rotationTotal:    rt,
		rotationDuration: rd,
	}, nil
}

func (o *keysObserver) RotationCheckStarted(ctx context.Context) (context.Context, keys.RotationCheckProbe) {
	return ctx, &rotationCheckProbe{metricProbe: metricProbe{
		ctx:       ctx,
		counter:   o.rotationTotal,
		histogram: o.rotationDuration,
		startTime: time.Now(),
	}}
}

type rotationCheckProbe struct {
	keys.NoOpRotationCheckProbe
	metricProbe
}

func (p *rotationCheckProbe) RotationCheckFailed(error) { p.markFailed() }
func (p *rotationCheckProbe) End()                      { p.record() }

var (
	_ keys.KeysObserver       = (*keysObserver)(nil)
	_ keys.RotationCheckProbe = (*rotationCheckProbe)(nil)
)
