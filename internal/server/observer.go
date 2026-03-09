package server

// JWKSObserver receives JWKS cache lifecycle events from JWKSServer.
// A nil observer means no events are emitted.
type JWKSObserver interface {
	InitialCachePopulationFailed(err error)
	CacheRefreshFailed(err error)
	KeyConversionFailed(keyID string, err error)
}

// ServerLifecycleObserver receives server lifecycle events from Server.
// A nil observer means no events are emitted.
type ServerLifecycleObserver interface {
	GRPCServeFailed(err error)
	HTTPServeFailed(err error)
}
