package server

import "context"

// JWKSObserver creates probes for JWKS cache refresh cycles.
type JWKSObserver interface {
	// JWKSCacheProbe returns a probe for one cache build or refresh (initial or background).
	JWKSCacheProbe(ctx context.Context) JWKSCacheProbe
}

// JWKSCacheProbe receives JWKS cache lifecycle events for a single refresh.
type JWKSCacheProbe interface {
	InitialCachePopulationFailed(err error)
	CacheRefreshFailed(err error)
	KeyConversionFailed(keyID string, err error)
}

// ServerLifecycleObserver creates probes for server listen/serve lifecycle.
type ServerLifecycleObserver interface {
	// ServerLifecycleProbe returns a probe for the server's serve goroutines.
	ServerLifecycleProbe(ctx context.Context) ServerLifecycleProbe
}

// ServerLifecycleProbe receives gRPC/HTTP serve failures for one server instance.
type ServerLifecycleProbe interface {
	GRPCServeFailed(err error)
	HTTPServeFailed(err error)
}

type noopJWKSCacheProbe struct{}

func (noopJWKSCacheProbe) InitialCachePopulationFailed(error) {}
func (noopJWKSCacheProbe) CacheRefreshFailed(error)           {}
func (noopJWKSCacheProbe) KeyConversionFailed(string, error)  {}

type noopServerLifecycleProbe struct{}

func (noopServerLifecycleProbe) GRPCServeFailed(error) {}
func (noopServerLifecycleProbe) HTTPServeFailed(error) {}

// NoopObserver satisfies both JWKSObserver and ServerLifecycleObserver
// with empty probes. Useful in tests that don't care about observer events.
type NoopObserver struct{}

func (NoopObserver) JWKSCacheProbe(context.Context) JWKSCacheProbe {
	return noopJWKSCacheProbe{}
}

func (NoopObserver) ServerLifecycleProbe(context.Context) ServerLifecycleProbe {
	return noopServerLifecycleProbe{}
}
