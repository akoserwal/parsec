package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/metrics"
	"github.com/project-kessel/parsec/internal/observer"
	"github.com/project-kessel/parsec/internal/probe"
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
func NewObserverWithLogger(cfg *ObservabilityConfig, logCtx LoggerContext) (observer.Observer, error) {
	return newObserver(cfg, logCtx, nil)
}

// NewObserverWithMetrics creates the central observer using the provided logger
// and an optional metrics.Provider. When the observer type is "metrics" and mp
// is non-nil, the provider is used to build a metrics-backed observer.
func NewObserverWithMetrics(cfg *ObservabilityConfig, logCtx LoggerContext, mp *metrics.Provider) (observer.Observer, error) {
	return newObserver(cfg, logCtx, mp)
}

func newObserver(cfg *ObservabilityConfig, logCtx LoggerContext, mp *metrics.Provider) (observer.Observer, error) {
	if cfg == nil {
		return observer.NoOp(), nil
	}

	switch cfg.Type {
	case "logging":
		return newLoggingObserver(cfg, logCtx)
	case "noop", "":
		return observer.NoOp(), nil
	case "metrics":
		return newMetricsObserver(mp)
	case "composite":
		return newCompositeObserver(cfg, logCtx, mp)
	default:
		return nil, fmt.Errorf("unknown observability type: %s (supported: logging, noop, metrics, composite)", cfg.Type)
	}
}

func newMetricsObserver(mp *metrics.Provider) (observer.Observer, error) {
	if mp == nil {
		return observer.NoOp(), nil
	}
	return metrics.NewObserver(mp)
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
// (both request-scoped and infra) to all child observers.
func newCompositeObserver(cfg *ObservabilityConfig, logCtx LoggerContext, mp *metrics.Provider) (observer.Observer, error) {
	if len(cfg.Observers) == 0 {
		return nil, fmt.Errorf("composite observer requires at least one sub-observer")
	}

	var children []observer.Observer
	for i, subCfg := range cfg.Observers {
		childLogCtx, err := deriveLoggerContext(logCtx, &subCfg)
		if err != nil {
			return nil, fmt.Errorf("observer %d: %w", i, err)
		}
		obs, err := newObserver(&subCfg, childLogCtx, mp)
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
