package config

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/httpfixture"
	"github.com/project-kessel/parsec/internal/metrics"
	"github.com/project-kessel/parsec/internal/observer"
	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

// Provider constructs all application components from configuration.
// Create one with NewProvider.
type Provider struct {
	config *Config

	logCtx          *LoggerContext
	obs             observer.Observer
	metricsProvider *metrics.Provider
	metricsErr      error
	metricsBuilt    bool

	// Lazily constructed components (cached after first call)
	trustStore           trust.Store
	dataSourceRegistry   *service.DataSourceRegistry
	issuerRegistry       service.Registry
	claimsFilterRegistry server.ClaimsFilterRegistry
	tokenService         *service.TokenService
	httpFixtureProvider  httpfixture.FixtureProvider
	httpFixtureBuilt     bool
}

// NewProvider creates a new provider from configuration.
func NewProvider(config *Config) *Provider {
	return &Provider{config: config}
}

func (p *Provider) loggerContext() (*LoggerContext, error) {
	if p.logCtx != nil {
		return p.logCtx, nil
	}

	lc, err := NewLoggerContext(p.config.Observability)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger context: %w", err)
	}

	p.logCtx = &lc
	return p.logCtx, nil
}

// Logger returns the application logger built from configuration.
func (p *Provider) Logger() (zerolog.Logger, error) {
	lc, err := p.loggerContext()
	if err != nil {
		return zerolog.Nop(), err
	}
	return lc.Logger, nil
}

// Observer returns the central observer, lazily created from configuration.
func (p *Provider) Observer() (observer.Observer, error) {
	if p.obs != nil {
		return p.obs, nil
	}

	lc, err := p.loggerContext()
	if err != nil {
		return nil, err
	}

	mp, err := p.MetricsProvider()
	if err != nil {
		return nil, err
	}

	obs, err := NewObserverWithMetrics(p.config.Observability, *lc, mp)
	if err != nil {
		return nil, fmt.Errorf("failed to create observer: %w", err)
	}

	p.obs = obs
	return obs, nil
}

// TrustStore returns the configured trust store.
func (p *Provider) TrustStore() (trust.Store, error) {
	if p.trustStore != nil {
		return p.trustStore, nil
	}

	obs, err := p.Observer()
	if err != nil {
		return nil, err
	}

	transport := p.HTTPTransport()
	store, err := NewTrustStore(p.config.TrustStore, transport, obs)
	if err != nil {
		return nil, fmt.Errorf("failed to create trust store: %w", err)
	}

	p.trustStore = store
	return store, nil
}

// DataSourceRegistry returns the configured data source registry
func (p *Provider) DataSourceRegistry() (*service.DataSourceRegistry, error) {
	if p.dataSourceRegistry != nil {
		return p.dataSourceRegistry, nil
	}

	obs, err := p.Observer()
	if err != nil {
		return nil, err
	}

	transport := p.HTTPTransport()
	registry, err := NewDataSourceRegistry(p.config.DataSources, transport, obs)
	if err != nil {
		return nil, fmt.Errorf("failed to create data source registry: %w", err)
	}

	p.dataSourceRegistry = registry
	return registry, nil
}

// IssuerRegistry returns the configured issuer registry
func (p *Provider) IssuerRegistry() (service.Registry, error) {
	if p.issuerRegistry != nil {
		return p.issuerRegistry, nil
	}

	obs, err := p.Observer()
	if err != nil {
		return nil, err
	}

	registry, err := NewIssuerRegistry(*p.config, obs)
	if err != nil {
		return nil, fmt.Errorf("failed to create issuer registry: %w", err)
	}

	p.issuerRegistry = registry
	return registry, nil
}

// ExchangeServerClaimsFilterRegistry returns the claims filter registry for the exchange server
func (p *Provider) ExchangeServerClaimsFilterRegistry() (server.ClaimsFilterRegistry, error) {
	if p.claimsFilterRegistry != nil {
		return p.claimsFilterRegistry, nil
	}

	// Get claims filter config from exchange server config
	var claimsFilterCfg ClaimsFilterConfig
	if p.config.ExchangeServer != nil {
		claimsFilterCfg = p.config.ExchangeServer.ClaimsFilter
	}

	registry, err := NewClaimsFilterRegistry(claimsFilterCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create claims filter registry: %w", err)
	}

	p.claimsFilterRegistry = registry
	return registry, nil
}

// TokenService returns the configured token service
func (p *Provider) TokenService() (*service.TokenService, error) {
	if p.tokenService != nil {
		return p.tokenService, nil
	}

	// Build dependencies
	dataSourceRegistry, err := p.DataSourceRegistry()
	if err != nil {
		return nil, err
	}

	issuerRegistry, err := p.IssuerRegistry()
	if err != nil {
		return nil, err
	}

	obs, err := p.Observer()
	if err != nil {
		return nil, err
	}

	tokenService := service.NewTokenService(
		p.config.TrustDomain,
		dataSourceRegistry,
		issuerRegistry,
		obs,
	)

	p.tokenService = tokenService
	return tokenService, nil
}

// MetricsProvider returns the metrics provider if metrics are enabled.
// Returns nil when metrics are not configured. The provider is lazily
// created and cached; a creation error is also cached so callers always
// see the same result.
func (p *Provider) MetricsProvider() (*metrics.Provider, error) {
	if p.metricsBuilt {
		return p.metricsProvider, p.metricsErr
	}
	p.metricsBuilt = true

	if !metricsEnabled(p.config.Observability) {
		return nil, nil
	}

	mp, err := metrics.New()
	if err != nil {
		p.metricsErr = fmt.Errorf("failed to create metrics provider: %w", err)
		return nil, p.metricsErr
	}
	p.metricsProvider = mp
	return mp, nil
}

// metricsEnabled returns true when the observability config uses metrics,
// either directly or via a composite child.
func metricsEnabled(cfg *ObservabilityConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.Type == "metrics" {
		return true
	}
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		return true
	}
	if cfg.Type == "composite" {
		for i := range cfg.Observers {
			if metricsEnabled(&cfg.Observers[i]) {
				return true
			}
		}
	}
	return false
}

// GRPCPort returns the configured gRPC port.
func (p *Provider) GRPCPort() int {
	return p.config.Server.GRPCPort
}

// HTTPPort returns the configured HTTP port.
func (p *Provider) HTTPPort() int {
	return p.config.Server.HTTPPort
}

// TrustDomain returns the configured trust domain
func (p *Provider) TrustDomain() string {
	return p.config.TrustDomain
}

// HTTPTransport returns an HTTP RoundTripper configured with fixtures if available
// Returns nil if no special transport is needed (caller should use http.DefaultTransport)
func (p *Provider) HTTPTransport() http.RoundTripper {
	fixtureProvider := p.HTTPFixtureProvider()
	if fixtureProvider == nil {
		return nil
	}
	return httpfixture.NewTransport(httpfixture.TransportConfig{
		Provider: fixtureProvider,
		Strict:   true,
	})
}

// HTTPFixtureProvider returns the fixture provider for hermetic testing
// Returns nil if no fixtures are configured (normal production mode)
func (p *Provider) HTTPFixtureProvider() httpfixture.FixtureProvider {
	if p.httpFixtureBuilt {
		return p.httpFixtureProvider
	}

	provider, err := BuildHTTPFixtureProvider(p.config.Fixtures, nil)
	if err != nil {
		// In production mode, fixture errors should fail fast
		// This is a configuration error, not a runtime error
		panic(fmt.Sprintf("failed to build HTTP fixture provider: %v", err))
	}

	p.httpFixtureProvider = provider
	p.httpFixtureBuilt = true
	return p.httpFixtureProvider
}

// AuthzServerTokenTypes returns the configured token types for ext_authz
func (p *Provider) AuthzServerTokenTypes() ([]server.TokenTypeSpec, error) {
	// If no authz server config, return nil (will use defaults)
	if p.config.AuthzServer == nil || len(p.config.AuthzServer.TokenTypes) == 0 {
		return nil, nil
	}

	var tokenTypes []server.TokenTypeSpec
	for _, ttCfg := range p.config.AuthzServer.TokenTypes {
		if ttCfg.Type == "" {
			return nil, fmt.Errorf("token type is required")
		}

		if ttCfg.HeaderName == "" {
			return nil, fmt.Errorf("header_name is required for token type %s", ttCfg.Type)
		}

		// Use token type directly as service.TokenType (it's already a URN string)
		tokenTypes = append(tokenTypes, server.TokenTypeSpec{
			Type:       service.TokenType(ttCfg.Type),
			HeaderName: ttCfg.HeaderName,
		})
	}

	return tokenTypes, nil
}
