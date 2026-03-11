package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/project-kessel/parsec/internal/config"
	"github.com/project-kessel/parsec/internal/probe"
	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

type infraEventConfigs struct {
	configReload    *config.EventLoggingConfig
	dataSourceCache *config.EventLoggingConfig
	keyRotation     *config.EventLoggingConfig
	keyProvider     *config.EventLoggingConfig
	trustValidation *config.EventLoggingConfig
	jwksCache       *config.EventLoggingConfig
	serverLifecycle *config.EventLoggingConfig
}

type wiredInfraObservers struct {
	configReload    *probe.LoggingConfigReloadObserver
	keyRotation     *probe.LoggingKeyRotationObserver
	keyProvider     *probe.LoggingKeyProviderObserver
	trustValidation *probe.LoggingTrustValidationObserver
	dataSourceCache *probe.LoggingDataSourceCacheObserver
	jwksCache       *probe.LoggingJWKSObserver
	serverLifecycle *probe.LoggingServerLifecycleObserver
}

type runtimeComponents struct {
	trustStore           trust.Store
	tokenService         *service.TokenService
	authzTokenTypes      []server.TokenTypeSpec
	claimsFilterRegistry server.ClaimsFilterRegistry
	issuerRegistry       service.Registry
}

func buildInfraEventConfigs(obs *config.ObservabilityConfig) infraEventConfigs {
	if obs == nil {
		return infraEventConfigs{}
	}
	return infraEventConfigs{
		configReload:    obs.ConfigReload,
		dataSourceCache: obs.DataSourceCache,
		keyRotation:     obs.KeyRotation,
		keyProvider:     obs.KeyProvider,
		trustValidation: obs.TrustValidation,
		jwksCache:       obs.JWKSCache,
		serverLifecycle: obs.ServerLifecycle,
	}
}

func wireInfraObservers(logCtx config.LoggerContext, infraCfg infraEventConfigs) wiredInfraObservers {
	return wiredInfraObservers{
		configReload: &probe.LoggingConfigReloadObserver{
			Logger: config.EventLogger(logCtx, "config_reload", infraCfg.configReload),
		},
		keyRotation: &probe.LoggingKeyRotationObserver{
			Logger: config.EventLogger(logCtx, "key_rotation", infraCfg.keyRotation),
		},
		keyProvider: &probe.LoggingKeyProviderObserver{
			Logger: config.EventLogger(logCtx, "key_provider", infraCfg.keyProvider),
		},
		trustValidation: &probe.LoggingTrustValidationObserver{
			Logger: config.EventLogger(logCtx, "trust_validation", infraCfg.trustValidation),
		},
		dataSourceCache: &probe.LoggingDataSourceCacheObserver{
			Logger: config.EventLogger(logCtx, "datasource_cache", infraCfg.dataSourceCache),
		},
		jwksCache: &probe.LoggingJWKSObserver{
			Logger: config.EventLogger(logCtx, "jwks_cache", infraCfg.jwksCache),
		},
		serverLifecycle: &probe.LoggingServerLifecycleObserver{
			Logger: config.EventLogger(logCtx, "server_lifecycle", infraCfg.serverLifecycle),
		},
	}
}

func buildRuntimeComponents(provider *config.Provider) (*runtimeComponents, error) {
	trustStore, err := provider.TrustStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create trust store: %w", err)
	}

	tokenService, err := provider.TokenService()
	if err != nil {
		return nil, fmt.Errorf("failed to create token service: %w", err)
	}

	authzTokenTypes, err := provider.AuthzServerTokenTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get authz token types: %w", err)
	}

	claimsFilterRegistry, err := provider.ExchangeServerClaimsFilterRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange server claims filter registry: %w", err)
	}

	issuerRegistry, err := provider.IssuerRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to get issuer registry: %w", err)
	}

	return &runtimeComponents{
		trustStore:           trustStore,
		tokenService:         tokenService,
		authzTokenTypes:      authzTokenTypes,
		claimsFilterRegistry: claimsFilterRegistry,
		issuerRegistry:       issuerRegistry,
	}, nil
}

// NewServeCmd creates the serve command
func NewServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the parsec server",
		Long: `Start the parsec gRPC and HTTP servers.

The server will:
  - Listen for gRPC requests (ext_authz, token exchange)
  - Listen for HTTP requests (token exchange via gRPC-gateway transcoding)
  - Load configuration from file, environment variables, and command-line flags

Configuration precedence (highest to lowest):
  1. Command-line flags
  2. Environment variables (PARSEC_*)
  3. Configuration file (if --config or PARSEC_CONFIG is set)
  4. Built-in defaults

Examples:
  # Start with default settings
  parsec serve

  # Override server ports
  parsec serve --server-grpc-port 9091 --server-http-port 8081

  # Override trust domain
  parsec serve --trust-domain prod.example.com

  # Use custom config file
  parsec serve --config /etc/parsec/config.yaml

  # Combine multiple overrides
  parsec serve --config ./my-config.yaml --server-grpc-port 9091`,
		RunE: runServe,
	}

	// Auto-register all config flags
	config.RegisterFlags(cmd.Flags())

	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Determine config file path
	configPath := configFile
	if configPath == "" {
		configPath = os.Getenv("PARSEC_CONFIG")
	}

	// 2. Load configuration (file + env vars + flags)
	loader, err := config.NewLoaderWithFlags(configPath, cmd.Flags())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg, err := loader.Get()
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// 3. Create provider to build all components from config
	provider := config.NewProvider(cfg)

	// 4. Create logger and observer — single instance shared across all components
	logCtx := config.NewLoggerContext(cfg.Observability)
	logger := logCtx.Logger
	infraCfg := buildInfraEventConfigs(cfg.Observability)
	infraObservers := wireInfraObservers(logCtx, infraCfg)
	loader.SetObserver(infraObservers.configReload)

	observer, err := config.NewObserverWithLogger(cfg.Observability, logCtx)
	if err != nil {
		return fmt.Errorf("failed to create observer: %w", err)
	}

	// Inject into provider so TokenService and other internal components use the same observer
	provider.SetObserver(observer)
	provider.SetKeyObservers(
		infraObservers.keyRotation,
		infraObservers.keyProvider,
	)
	provider.SetTrustObserver(infraObservers.trustValidation)
	provider.SetCacheObserver(infraObservers.dataSourceCache)

	// 5. Build components via provider
	components, err := buildRuntimeComponents(provider)
	if err != nil {
		return err
	}

	// 6. Create service handlers with observability
	authzServer := server.NewAuthzServer(components.trustStore, components.tokenService, components.authzTokenTypes, observer)
	exchangeServer := server.NewExchangeServer(components.trustStore, components.tokenService, components.claimsFilterRegistry, observer)
	jwksServer := server.NewJWKSServer(server.JWKSServerConfig{
		IssuerRegistry: components.issuerRegistry,
		Observer:       infraObservers.jwksCache,
	})

	// Start JWKS background refresh
	if err := jwksServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start JWKS server: %w", err)
	}
	defer jwksServer.Stop()

	// 7. Create TCP listeners from configured ports
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", provider.GRPCPort()))
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port %d: %w", provider.GRPCPort(), err)
	}
	defer func() { _ = grpcListener.Close() }()

	httpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", provider.HTTPPort()))
	if err != nil {
		return fmt.Errorf("failed to listen on HTTP port %d: %w", provider.HTTPPort(), err)
	}
	defer func() { _ = httpListener.Close() }()

	// 8. Create and start server
	srv := server.New(server.Config{
		GRPCListener:   grpcListener,
		HTTPListener:   httpListener,
		AuthzServer:    authzServer,
		ExchangeServer: exchangeServer,
		JWKSServer:     jwksServer,
		Observer:       infraObservers.serverLifecycle,
	})
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// 8a. All components initialized — signal readiness via gRPC health service.
	srv.SetReady()

	grpcAddr := grpcListener.Addr().String()
	httpAddr := httpListener.Addr().String()
	logger.Info().
		Str("grpc_addr", grpcAddr).
		Str("http_addr", httpAddr).
		Str("token_exchange_url", "http://"+httpAddr+"/v1/token").
		Str("jwks_url", "http://"+httpAddr+"/v1/jwks.json").
		Str("jwks_wellknown_url", "http://"+httpAddr+"/.well-known/jwks.json").
		Str("health_grpc", grpcAddr+" (grpc.health.v1.Health)").
		Str("trust_domain", provider.TrustDomain()).
		Str("config", configPath).
		Msg("parsec is running")

	// 9. Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info().Msg("Shutting down")

	// 10. Graceful shutdown
	if err := srv.Stop(ctx); err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}

	logger.Info().Msg("Shutdown complete")
	return nil
}
