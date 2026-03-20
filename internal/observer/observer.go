package observer

import (
	"context"

	"github.com/project-kessel/parsec/internal/datasource"
	"github.com/project-kessel/parsec/internal/keys"
	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

// Observer is the central observability interface for the entire application.
// All components that need observability receive this single type rather than
// individually-typed observer interfaces. It composes the application-level
// observer (token issuance, exchange, authz) with all infrastructure observers.
//
// Config reload logging is intentionally excluded: the Loader is a bootstrap
// component; it logs reload failures directly via zerolog (Loader.SetReloadLogger)
// after the first successful config parse.
type Observer interface {
	service.ApplicationObserver
	datasource.DataSourceCacheObserver
	datasource.LuaDataSourceObserver
	keys.KeyRotationObserver
	keys.KeyProviderObserver
	trust.TrustValidationObserver
	server.JWKSObserver
	server.ServerLifecycleObserver
}

// composed holds individually-constructed observers and satisfies Observer
// by promoting all embedded interface methods.
type composed struct {
	service.ApplicationObserver
	datasource.DataSourceCacheObserver
	datasource.LuaDataSourceObserver
	keys.KeyRotationObserver
	keys.KeyProviderObserver
	trust.TrustValidationObserver
	server.JWKSObserver
	server.ServerLifecycleObserver
}

// Compose builds an Observer from individually-constructed sub-observers.
// Use this during bootstrap when each sub-observer is created separately
// (e.g., with different loggers or config-driven log levels).
func Compose(
	app service.ApplicationObserver,
	cache datasource.DataSourceCacheObserver,
	luaDS datasource.LuaDataSourceObserver,
	keyRotation keys.KeyRotationObserver,
	keyProvider keys.KeyProviderObserver,
	trustObs trust.TrustValidationObserver,
	jwks server.JWKSObserver,
	serverLifecycle server.ServerLifecycleObserver,
) Observer {
	return &composed{
		ApplicationObserver:     app,
		DataSourceCacheObserver: cache,
		LuaDataSourceObserver:   luaDS,
		KeyRotationObserver:     keyRotation,
		KeyProviderObserver:     keyProvider,
		TrustValidationObserver: trustObs,
		JWKSObserver:            jwks,
		ServerLifecycleObserver: serverLifecycle,
	}
}

// Noop returns an Observer where every method is a no-op.
func Noop() Observer {
	return &noopObserver{}
}

// noopObserver satisfies Observer with empty methods and no-op probes.
type noopObserver struct{}

// ApplicationObserver methods

func (n *noopObserver) TokenIssuanceStarted(ctx context.Context, _ *trust.Result, _ *trust.Result, _ string, _ []service.TokenType) (context.Context, service.TokenIssuanceProbe) {
	return ctx, &service.NoOpTokenIssuanceProbe{}
}

func (n *noopObserver) TokenExchangeStarted(ctx context.Context, _ string, _ string, _ string, _ string) (context.Context, service.TokenExchangeProbe) {
	return ctx, &service.NoOpTokenExchangeProbe{}
}

func (n *noopObserver) AuthzCheckStarted(ctx context.Context) (context.Context, service.AuthzCheckProbe) {
	return ctx, &service.NoOpAuthzCheckProbe{}
}

// DataSourceCacheObserver

type noopDSCProbe struct{}

func (noopDSCProbe) CacheHit()         {}
func (noopDSCProbe) CacheMiss()        {}
func (noopDSCProbe) CacheExpired()     {}
func (noopDSCProbe) FetchFailed(error) {}

func (n *noopObserver) DataSourceCacheProbe(context.Context, string) datasource.DataSourceCacheProbe {
	return noopDSCProbe{}
}

// LuaDataSourceObserver

type noopLuaDSProbe struct{}

func (noopLuaDSProbe) ScriptLoadFailed(error)      {}
func (noopLuaDSProbe) ScriptExecutionFailed(error) {}
func (noopLuaDSProbe) InvalidReturnType(string)    {}
func (noopLuaDSProbe) FetchCompleted()             {}

func (n *noopObserver) LuaDataSourceProbe(context.Context, string) datasource.LuaDataSourceProbe {
	return noopLuaDSProbe{}
}

// KeyRotationObserver

type noopKeyRotProbe struct{}

func (noopKeyRotProbe) RotationCheckFailed(error)          {}
func (noopKeyRotProbe) ActiveKeyCacheUpdateFailed(error)   {}
func (noopKeyRotProbe) RotationCompleted(string)           {}
func (noopKeyRotProbe) RotationSkippedVersionRace(string)  {}
func (noopKeyRotProbe) KeyProviderNotFound(string, string) {}
func (noopKeyRotProbe) KeyHandleFailed(string, error)      {}
func (noopKeyRotProbe) PublicKeyFailed(string, error)      {}
func (noopKeyRotProbe) ThumbprintFailed(string, error)     {}
func (noopKeyRotProbe) MetadataFailed(string, error)       {}

func (n *noopObserver) KeyRotationProbe(context.Context) keys.KeyRotationProbe {
	return noopKeyRotProbe{}
}

// KeyProviderObserver

type noopKeyProvProbe struct{}

func (noopKeyProvProbe) OldKeyDeletionFailed(string, error) {}

func (n *noopObserver) KeyProviderProbe(context.Context) keys.KeyProviderProbe {
	return noopKeyProvProbe{}
}

// TrustValidationObserver

type noopTrustProbe struct{}

func (noopTrustProbe) ValidatorFailed(string, trust.CredentialType, error)  {}
func (noopTrustProbe) AllValidatorsFailed(trust.CredentialType, int, error) {}
func (noopTrustProbe) ValidatorFiltered(string, string)                     {}
func (noopTrustProbe) FilterEvaluationFailed(string, error)                 {}

func (n *noopObserver) TrustValidationProbe(context.Context) trust.TrustValidationProbe {
	return noopTrustProbe{}
}

// JWKSObserver

type noopJWKSProbe struct{}

func (noopJWKSProbe) InitialCachePopulationFailed(error) {}
func (noopJWKSProbe) CacheRefreshFailed(error)           {}
func (noopJWKSProbe) KeyConversionFailed(string, error)  {}

func (n *noopObserver) JWKSCacheProbe(context.Context) server.JWKSCacheProbe {
	return noopJWKSProbe{}
}

// ServerLifecycleObserver

type noopSrvLifeProbe struct{}

func (noopSrvLifeProbe) GRPCServeFailed(error) {}
func (noopSrvLifeProbe) HTTPServeFailed(error) {}

func (n *noopObserver) ServerLifecycleProbe(context.Context) server.ServerLifecycleProbe {
	return noopSrvLifeProbe{}
}

// Compile-time check: both implementations satisfy Observer.
var (
	_ Observer = (*composed)(nil)
	_ Observer = (*noopObserver)(nil)
)
