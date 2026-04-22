package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/project-kessel/parsec/internal/observer"
	"github.com/project-kessel/parsec/internal/probe"
	"github.com/project-kessel/parsec/internal/version"
)

// LoggerContext couples a zerolog logger with its destination writer so
// per-event formatting overrides can preserve the original sink.
type LoggerContext struct {
	Logger zerolog.Logger
	Writer io.Writer
}

// NewObserver creates the central observer from configuration.
// This is a convenience wrapper that creates its own logger from cfg.
func NewObserver(cfg *ObservabilityConfig) (observer.Observer, error) {
	logCtx, err := NewLoggerContext(cfg)
	if err != nil {
		return nil, err
	}
	return NewObserverWithLogger(cfg, logCtx)
}

// NewObserverWithLogger creates the central observer using the provided logger.
// Use this when you want the observer to share a logger with other components.
// The returned Observer satisfies both ServiceObserver and all infra
// observer interfaces (cache, keys, trust, JWKS, server lifecycle).
//
// If the config includes prometheus (directly or inside a composite), a
// MeterProvider is created automatically. Use Provider.Observer() when
// you need the MeterProvider shared with transport-level instrumentation.
func NewObserverWithLogger(cfg *ObservabilityConfig, logCtx LoggerContext) (observer.Observer, error) {
	mp, _, err := buildMetricsIfConfigured(cfg)
	if err != nil {
		return nil, err
	}
	return newObserverWithMetrics(cfg, logCtx, mp)
}

// newObserverWithMetrics is the internal constructor that threads a
// MeterProvider to the "prometheus" observer case. Callers must build
// the MeterProvider beforehand (via buildMetricsIfConfigured); passing
// nil when the config includes prometheus is an error.
func newObserverWithMetrics(cfg *ObservabilityConfig, logCtx LoggerContext, mp *sdkmetric.MeterProvider) (observer.Observer, error) {
	if cfg == nil {
		return observer.NoOp(), nil
	}

	switch cfg.Type {
	case "logging":
		return newLoggingObserver(cfg, logCtx)
	case "prometheus":
		return newPrometheusObserver(mp)
	case "noop", "":
		return observer.NoOp(), nil
	case "composite":
		return newCompositeObserver(cfg, logCtx, mp)
	default:
		return nil, fmt.Errorf("unknown observability type: %s (supported: logging, noop, prometheus, composite)", cfg.Type)
	}
}

// newPrometheusObserver builds a metrics-only observer from the given
// MeterProvider. The caller (NewObserverWithLogger or Provider.Observer)
// is responsible for building the MeterProvider before calling this.
func newPrometheusObserver(mp *sdkmetric.MeterProvider) (observer.Observer, error) {
	if mp == nil {
		return nil, fmt.Errorf("prometheus observer requires a MeterProvider; use Provider.Observer() or supply one via composite config")
	}
	return probe.NewMetricsObserver(mp), nil
}

// NewLogger creates a structured zerolog logger from the observability configuration.
func NewLogger(cfg *ObservabilityConfig) (zerolog.Logger, error) {
	logCtx, err := NewLoggerContext(cfg)
	if err != nil {
		return zerolog.Logger{}, err
	}
	return logCtx.Logger, nil
}

func newLoggingObserver(cfg *ObservabilityConfig, logCtx LoggerContext) (observer.Observer, error) {
	el := func(name string, ecfg *EventLoggingConfig) (zerolog.Logger, error) {
		return EventLogger(logCtx, name, ecfg)
	}

	tiLog, err := el("token_issuance", cfg.TokenIssuance)
	if err != nil {
		return nil, err
	}
	teLog, err := el("token_exchange", cfg.TokenExchange)
	if err != nil {
		return nil, err
	}
	acLog, err := el("authz_check", cfg.AuthzCheck)
	if err != nil {
		return nil, err
	}
	dcLog, err := el("datasource_cache", cfg.DataSourceCache)
	if err != nil {
		return nil, err
	}
	luaLog, err := el("lua_datasource", cfg.LuaDataSource)
	if err != nil {
		return nil, err
	}
	krLog, err := el("key_rotation", cfg.KeyRotation)
	if err != nil {
		return nil, err
	}
	kpLog, err := el("key_provider", cfg.KeyProvider)
	if err != nil {
		return nil, err
	}
	tvLog, err := el("trust_validation", cfg.TrustValidation)
	if err != nil {
		return nil, err
	}
	jcLog, err := el("jwks_cache", cfg.JWKSCache)
	if err != nil {
		return nil, err
	}
	slLog, err := el("server_lifecycle", cfg.ServerLifecycle)
	if err != nil {
		return nil, err
	}

	app := probe.NewLoggingObserverWithConfig(probe.LoggingObserverConfig{
		TokenIssuanceLogger: tiLog,
		TokenExchangeLogger: teLog,
		AuthzCheckLogger:    acLog,
	})

	return observer.Compose(
		app,
		probe.NewLoggingDataSourceObserver(dcLog, luaLog),
		probe.NewLoggingKeysObserver(krLog, kpLog),
		probe.NewLoggingTrustObserver(tvLog),
		probe.NewLoggingServerObserver(jcLog, slLog),
	), nil
}

// NewLoggerContext creates a structured zerolog logger and the writer used as
// its sink. Writer holds the raw destination (e.g. os.Stdout), never a
// format wrapper, so EventLogger can re-wrap it with a different format.
func NewLoggerContext(cfg *ObservabilityConfig) (LoggerContext, error) {
	if cfg == nil {
		return LoggerContext{
			Logger: zerolog.New(os.Stdout).With().Timestamp().Logger(),
			Writer: os.Stdout,
		}, nil
	}

	rawSink := os.Stdout
	defaultLevel, err := parseLogLevel(cfg.LogLevel)
	if err != nil {
		return LoggerContext{}, err
	}
	writer, err := createWriter(cfg.LogFormat, rawSink)
	if err != nil {
		return LoggerContext{}, err
	}
	return LoggerContext{
		Logger: zerolog.New(writer).With().Timestamp().Logger().Level(defaultLevel),
		Writer: rawSink,
	}, nil
}

// newCompositeObserver creates a composite observer that fans out every call
// (both request-scoped and infra) to all child observers. A shared
// MeterProvider is threaded to any "prometheus" children.
func newCompositeObserver(cfg *ObservabilityConfig, logCtx LoggerContext, mp *sdkmetric.MeterProvider) (observer.Observer, error) {
	if len(cfg.Observers) == 0 {
		return nil, fmt.Errorf("composite observer requires at least one sub-observer")
	}

	var children []observer.Observer
	for i, subCfg := range cfg.Observers {
		childLogCtx, err := deriveLoggerContext(logCtx, &subCfg)
		if err != nil {
			return nil, fmt.Errorf("observer %d: %w", i, err)
		}
		obs, err := newObserverWithMetrics(&subCfg, childLogCtx, mp)
		if err != nil {
			return nil, fmt.Errorf("failed to create observer %d: %w", i, err)
		}
		children = append(children, obs)
	}

	return observer.CompositeAll(children), nil
}

// deriveLoggerContext builds a child LoggerContext that shares the parent's
// raw sink but applies the child config's LogLevel and/or LogFormat overrides.
// If the child specifies neither, the parent context is returned as-is.
func deriveLoggerContext(parent LoggerContext, cfg *ObservabilityConfig) (LoggerContext, error) {
	if cfg == nil || (cfg.LogLevel == "" && cfg.LogFormat == "") {
		return parent, nil
	}

	logger := parent.Logger

	if cfg.LogFormat != "" {
		w, err := createWriter(cfg.LogFormat, parent.Writer)
		if err != nil {
			return LoggerContext{}, err
		}
		logger = logger.Output(w)
	}
	if cfg.LogLevel != "" {
		lvl, err := parseLogLevel(cfg.LogLevel)
		if err != nil {
			return LoggerContext{}, err
		}
		logger = logger.Level(lvl)
	}

	return LoggerContext{
		Logger: logger,
		Writer: parent.Writer,
	}, nil
}

// EventLogger creates a pre-configured sub-logger for a specific event type.
// The returned logger has the "event" field baked in.
//
// Override precedence (applied in order):
//  1. LogFormat -- output format is always applied first
//  2. Enabled=false -- disables the event entirely (zerolog.Disabled), overrides LogLevel
//  3. LogLevel -- sets the minimum severity threshold
//
// If eventCfg is nil the logger inherits all base settings unchanged.
func EventLogger(logCtx LoggerContext, eventName string, eventCfg *EventLoggingConfig) (zerolog.Logger, error) {
	logger := logCtx.Logger.With().Str("event", eventName).Logger()
	if eventCfg == nil {
		return logger, nil
	}
	if eventCfg.LogFormat != "" {
		w, err := createWriter(eventCfg.LogFormat, logCtx.Writer)
		if err != nil {
			return zerolog.Logger{}, fmt.Errorf("%s: %w", eventName, err)
		}
		logger = logger.Output(w)
	}
	if eventCfg.Enabled != nil && !*eventCfg.Enabled {
		return logger.Level(zerolog.Disabled), nil
	}
	if eventCfg.LogLevel != "" {
		lvl, err := parseLogLevel(eventCfg.LogLevel)
		if err != nil {
			return zerolog.Logger{}, fmt.Errorf("%s: %w", eventName, err)
		}
		return logger.Level(lvl), nil
	}
	return logger, nil
}

func createWriter(format string, fallback io.Writer) (io.Writer, error) {
	if fallback == nil {
		fallback = os.Stdout
	}
	switch strings.ToLower(format) {
	case "text":
		return zerolog.ConsoleWriter{Out: fallback}, nil
	case "json", "":
		return fallback, nil
	default:
		return nil, fmt.Errorf("invalid log_format %q (valid: json, text)", format)
	}
}

func parseLogLevel(levelStr string) (zerolog.Level, error) {
	switch strings.ToLower(levelStr) {
	case "debug":
		return zerolog.DebugLevel, nil
	case "info", "":
		return zerolog.InfoLevel, nil
	case "warn", "warning":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	default:
		return 0, fmt.Errorf("invalid log_level %q (valid: debug, info, warn, error)", levelStr)
	}
}

// buildMetricsIfConfigured walks the ObservabilityConfig tree. If a
// "prometheus" type is found (at the top level or inside a composite),
// it creates a single shared MeterProvider and promhttp.Handler.
// Returns (nil, nil, nil) when metrics are not configured.
func buildMetricsIfConfigured(cfg *ObservabilityConfig) (*sdkmetric.MeterProvider, http.Handler, error) {
	if !hasPrometheus(cfg) {
		return nil, nil, nil
	}

	exporter, err := promexporter.New()
	if err != nil {
		return nil, nil, fmt.Errorf("prometheus exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("parsec"),
			semconv.ServiceVersion(version.Version),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otel resource: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(res),
		sdkmetric.WithView(histogramViews()...),
	)

	return mp, promhttp.Handler(), nil
}

// hasPrometheus returns true if cfg (or any composite child) has type "prometheus".
func hasPrometheus(cfg *ObservabilityConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.Type == "prometheus" {
		return true
	}
	for i := range cfg.Observers {
		if hasPrometheus(&cfg.Observers[i]) {
			return true
		}
	}
	return false
}

// histogramViews returns OTel Views that apply custom bucket boundaries
// for parsec-specific histogram instruments.
func histogramViews() []sdkmetric.View {
	requestPathBuckets := []float64{
		0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5,
	}
	cacheBuckets := []float64{
		0.00001, 0.00005, 0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05,
	}
	externalCallBuckets := []float64{
		0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
	}
	shutdownBuckets := []float64{
		0.1, 0.5, 1, 2.5, 5, 10, 30,
	}

	mkView := func(namePattern string, boundaries []float64) sdkmetric.View {
		return sdkmetric.NewView(
			sdkmetric.Instrument{Name: namePattern},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: boundaries,
			}},
		)
	}

	return []sdkmetric.View{
		mkView("parsec.token.*duration", requestPathBuckets),
		mkView("parsec.authz.*duration", requestPathBuckets),
		mkView("parsec.trust.*duration", requestPathBuckets),
		mkView("parsec.jwt.*duration", requestPathBuckets),
		mkView("parsec.datasource.cache.duration", cacheBuckets),
		mkView("parsec.datasource.lua.*duration", externalCallBuckets),
		mkView("parsec.jwks.*duration", externalCallBuckets),
		mkView("parsec.key.*duration", externalCallBuckets),
		mkView("parsec.server.shutdown.duration", shutdownBuckets),
	}
}
