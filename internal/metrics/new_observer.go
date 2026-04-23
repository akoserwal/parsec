package metrics

import (
	"github.com/project-kessel/parsec/internal/observer"
)

// NewObserver builds a full observer.Observer backed by OTel metrics.
// The returned observer satisfies all per-package aggregate interfaces
// and records counters/histograms via the given Provider.
func NewObserver(p *Provider) (observer.Observer, error) {
	m := p.Meter(meterName)

	svc, err := newServiceObserver(m)
	if err != nil {
		return nil, err
	}
	ds, err := newDataSourceObserver(m)
	if err != nil {
		return nil, err
	}
	ks, err := newKeysObserver(m)
	if err != nil {
		return nil, err
	}
	ts, err := newTrustObserver(m)
	if err != nil {
		return nil, err
	}
	srv := newServerObserver()

	return observer.Compose(svc, ds, ks, ts, srv), nil
}
