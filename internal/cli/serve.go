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
	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

type runtimeComponents struct {
	trustStore           trust.Store
	tokenService         *service.TokenService
	authzTokenTypes      []server.TokenTypeSpec
	claimsFilterRegistry server.ClaimsFilterRegistry
	issuerRegistry       service.Registry
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

	// 3. Create provider (lazily builds logger and observer from config)
	provider := config.NewProvider(cfg)

	logger, err := provider.Logger()
	if err != nil {
		return fmt.Errorf("invalid observability config: %w", err)
	}

	obs, err := provider.Observer()
	if err != nil {
		return fmt.Errorf("failed to create observer: %w", err)
	}

	// 4. Build components via provider
	components, err := buildRuntimeComponents(provider)
	if err != nil {
		return err
	}

	// 5. Create service handlers — all receive the central observer
	authzServer := server.NewAuthzServer(components.trustStore, components.tokenService, components.authzTokenTypes, obs)
	exchangeServer := server.NewExchangeServer(components.trustStore, components.tokenService, components.claimsFilterRegistry, obs)
	jwksServer := server.NewJWKSServer(server.JWKSServerConfig{
		IssuerRegistry: components.issuerRegistry,
		Observer:       obs,
	})

	// Start JWKS background refresh
	if err := jwksServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start JWKS server: %w", err)
	}
	defer jwksServer.Stop()

	// 6. Create TCP listeners from configured ports
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

	// 7. Create and start server
	srv := server.New(server.Config{
		GRPCListener:   grpcListener,
		HTTPListener:   httpListener,
		AuthzServer:    authzServer,
		ExchangeServer: exchangeServer,
		JWKSServer:     jwksServer,
		Observer:       obs,
	})
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// 7a. All components initialized — signal readiness via gRPC health service.
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

	// 8. Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info().Msg("Shutting down")

	// 9. Graceful shutdown
	if err := srv.Stop(ctx); err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}

	logger.Info().Msg("Shutdown complete")
	return nil
}
