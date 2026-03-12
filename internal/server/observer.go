package server

// JWKSObserver receives JWKS cache lifecycle events from JWKSServer.
type JWKSObserver interface {
	InitialCachePopulationFailed(err error)
	CacheRefreshFailed(err error)
	KeyConversionFailed(keyID string, err error)
}

// ServerLifecycleObserver receives server lifecycle events from Server.
type ServerLifecycleObserver interface {
	GRPCServeFailed(err error)
	HTTPServeFailed(err error)
}

// NoopObserver satisfies both JWKSObserver and ServerLifecycleObserver
// with empty methods. Useful in tests that don't care about observer events.
type NoopObserver struct{}

func (NoopObserver) InitialCachePopulationFailed(error) {}
func (NoopObserver) CacheRefreshFailed(error)           {}
func (NoopObserver) KeyConversionFailed(string, error)  {}
func (NoopObserver) GRPCServeFailed(error)              {}
func (NoopObserver) HTTPServeFailed(error)              {}
