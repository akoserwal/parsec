package server

import "context"

// JWKSObserver is called at key points during JWKS cache operations.
// Implementations should embed NoOpJWKSObserver for forward compatibility
// with new methods added to this interface.
type JWKSObserver interface {
	// CacheRefreshStarted is called when a JWKS cache build or refresh begins.
	// Returns a potentially modified context and a probe to track the operation.
	CacheRefreshStarted(ctx context.Context) (context.Context, CacheRefreshProbe)
}

// CacheRefreshProbe tracks a single JWKS cache refresh invocation.
// Implementations should embed NoOpCacheRefreshProbe for forward compatibility.
type CacheRefreshProbe interface {
	InitialCachePopulationFailed(err error)
	CacheRefreshFailed(err error)
	KeyConversionFailed(keyID string, err error)
	End()
}

// LifecycleObserver is called at key points during server lifecycle.
// Implementations should embed NoOpLifecycleObserver for forward compatibility
// with new methods added to this interface.
type LifecycleObserver interface {
	// ServeStarted is called when the server begins serving.
	// Returns a potentially modified context and a probe to track the lifecycle.
	ServeStarted(ctx context.Context) (context.Context, ServeProbe)
}

// ServeProbe tracks server lifecycle events for one server instance.
// Implementations should embed NoOpServeProbe for forward compatibility.
type ServeProbe interface {
	GRPCServeFailed(err error)
	HTTPServeFailed(err error)
	End()
}

// ServerObserver is the per-package aggregate for all server observer interfaces.
type ServerObserver interface {
	JWKSObserver
	LifecycleObserver
}

// --- NoOp implementations ---

// NoOpCacheRefreshProbe is a no-op implementation of CacheRefreshProbe.
// Embed this in concrete probe types for forward compatibility.
type NoOpCacheRefreshProbe struct{}

func (NoOpCacheRefreshProbe) InitialCachePopulationFailed(error) {}
func (NoOpCacheRefreshProbe) CacheRefreshFailed(error)           {}
func (NoOpCacheRefreshProbe) KeyConversionFailed(string, error)  {}
func (NoOpCacheRefreshProbe) End()                               {}

// NoOpServeProbe is a no-op implementation of ServeProbe.
// Embed this in concrete probe types for forward compatibility.
type NoOpServeProbe struct{}

func (NoOpServeProbe) GRPCServeFailed(error) {}
func (NoOpServeProbe) HTTPServeFailed(error) {}
func (NoOpServeProbe) End()                  {}

// NoOpObserver satisfies both JWKSObserver and LifecycleObserver
// with empty probes. Useful in tests that don't care about observer events.
type NoOpObserver struct{}

func (NoOpObserver) CacheRefreshStarted(ctx context.Context) (context.Context, CacheRefreshProbe) {
	return ctx, NoOpCacheRefreshProbe{}
}

func (NoOpObserver) ServeStarted(ctx context.Context) (context.Context, ServeProbe) {
	return ctx, NoOpServeProbe{}
}

var _ ServerObserver = NoOpObserver{}
