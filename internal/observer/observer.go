package observer

import (
	"github.com/project-kessel/parsec/internal/datasource"
	"github.com/project-kessel/parsec/internal/keys"
	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

// Observer is the central observability interface for the entire application.
// It embeds per-package aggregate observer interfaces so that adding a new
// observer to a domain package only requires updating that package's Observer
// — not this central type.
//
// An Observer value is assignable to any narrower per-package or per-operation
// observer interface (e.g. service.ServiceObserver, datasource.CacheObserver,
// keys.DualSlotRotatingSignerObserver) via Go structural typing.
//
// Config reload logging is intentionally excluded from the Observer interface.
type Observer interface {
	service.ServiceObserver
	datasource.DataSourceObserver
	keys.KeysObserver
	trust.TrustObserver
	server.ServerObserver
}

// composed holds per-package aggregate observers and satisfies Observer
// by promoting all embedded interface methods.
type composed struct {
	service.ServiceObserver
	datasource.DataSourceObserver
	keys.KeysObserver
	trust.TrustObserver
	server.ServerObserver
}

// Compose builds an Observer from per-package aggregate observers.
func Compose(
	app service.ServiceObserver,
	ds datasource.DataSourceObserver,
	ks keys.KeysObserver,
	ts trust.TrustObserver,
	srv server.ServerObserver,
) Observer {
	return &composed{
		ServiceObserver:    app,
		DataSourceObserver: ds,
		KeysObserver:       ks,
		TrustObserver:      ts,
		ServerObserver:     srv,
	}
}

// NoOp returns an Observer where every method is a no-op.
func NoOp() Observer {
	return &noopObserver{}
}

// Type aliases disambiguate the identical NoOpObserver names from different
// packages so they can all be embedded anonymously in a single struct.
type (
	noopDatasource = datasource.NoOpObserver
	noopKeys       = keys.NoOpObserver
	noopTrust      = trust.NoOpObserver
	noopServer     = server.NoOpObserver
)

// noopObserver satisfies Observer by embedding per-package NoOp types.
// All methods are promoted automatically — no hand-written forwarding needed.
type noopObserver struct {
	service.NoOpServiceObserver
	noopDatasource
	noopKeys
	noopTrust
	noopServer
}

// Compile-time check: both implementations satisfy Observer.
var (
	_ Observer = (*composed)(nil)
	_ Observer = (*noopObserver)(nil)
)
