package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"

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
	return NewObserverWithLogger(cfg, NewLoggerContext(cfg))
}

// NewObserverWithLogger creates the central observer using the provided logger.
// Use this when you want the observer to share a logger with other components.
// The returned Observer satisfies both ApplicationObserver and all infra
// observer interfaces (cache, keys, trust, JWKS, server lifecycle).
func NewObserverWithLogger(cfg *ObservabilityConfig, logCtx LoggerContext) (observer.Observer, error) {
	if cfg == nil {
		return observer.Noop(), nil
	}

	switch cfg.Type {
	case "logging":
		return newLoggingObserver(cfg, logCtx), nil
	case "noop", "":
		return observer.Noop(), nil
	case "composite":
		return newCompositeObserver(cfg, logCtx)
	default:
		return nil, fmt.Errorf("unknown observability type: %s (supported: logging, noop, composite)", cfg.Type)
	}
}

// NewLogger creates a structured zerolog logger from the observability configuration.
func NewLogger(cfg *ObservabilityConfig) zerolog.Logger {
	return NewLoggerContext(cfg).Logger
}

func newLoggingObserver(cfg *ObservabilityConfig, logCtx LoggerContext) observer.Observer {
	app := probe.NewLoggingObserverWithConfig(probe.LoggingObserverConfig{
		TokenIssuanceLogger: EventLogger(logCtx, "token_issuance", cfg.TokenIssuance),
		TokenExchangeLogger: EventLogger(logCtx, "token_exchange", cfg.TokenExchange),
		AuthzCheckLogger:    EventLogger(logCtx, "authz_check", cfg.AuthzCheck),
	})

	return observer.Compose(
		app,
		probe.NewLoggingDataSourceCacheObserver(EventLogger(logCtx, "datasource_cache", cfg.DataSourceCache)),
		probe.NewLoggingLuaDataSourceObserver(EventLogger(logCtx, "lua_datasource", cfg.LuaDataSource)),
		probe.NewLoggingKeyRotationObserver(EventLogger(logCtx, "key_rotation", cfg.KeyRotation)),
		probe.NewLoggingKeyProviderObserver(EventLogger(logCtx, "key_provider", cfg.KeyProvider)),
		probe.NewLoggingTrustValidationObserver(EventLogger(logCtx, "trust_validation", cfg.TrustValidation)),
		probe.NewLoggingJWKSObserver(EventLogger(logCtx, "jwks_cache", cfg.JWKSCache)),
		probe.NewLoggingServerLifecycleObserver(EventLogger(logCtx, "server_lifecycle", cfg.ServerLifecycle)),
	)
}

// NewLoggerContext creates a structured zerolog logger and the writer used as
// its sink. Writer holds the raw destination (e.g. os.Stdout), never a
// format wrapper, so EventLogger can re-wrap it with a different format.
func NewLoggerContext(cfg *ObservabilityConfig) LoggerContext {
	if cfg == nil {
		return LoggerContext{
			Logger: zerolog.New(os.Stdout).With().Timestamp().Logger(),
			Writer: os.Stdout,
		}
	}

	rawSink := os.Stdout
	defaultLevel := parseLogLevel(cfg.LogLevel)
	writer := createWriter(cfg.LogFormat, rawSink)
	return LoggerContext{
		Logger: zerolog.New(writer).With().Timestamp().Logger().Level(defaultLevel),
		Writer: rawSink,
	}
}

// newCompositeObserver creates a composite observer that fans out every call
// (both request-scoped and infra) to all child observers.
func newCompositeObserver(cfg *ObservabilityConfig, logCtx LoggerContext) (observer.Observer, error) {
	if len(cfg.Observers) == 0 {
		return nil, fmt.Errorf("composite observer requires at least one sub-observer")
	}

	var children []observer.Observer
	for i, subCfg := range cfg.Observers {
		childLogCtx := deriveLoggerContext(logCtx, &subCfg)
		obs, err := NewObserverWithLogger(&subCfg, childLogCtx)
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
func deriveLoggerContext(parent LoggerContext, cfg *ObservabilityConfig) LoggerContext {
	if cfg == nil || (cfg.LogLevel == "" && cfg.LogFormat == "") {
		return parent
	}

	logger := parent.Logger

	if cfg.LogFormat != "" {
		logger = logger.Output(createWriter(cfg.LogFormat, parent.Writer))
	}
	if cfg.LogLevel != "" {
		logger = logger.Level(parseLogLevel(cfg.LogLevel))
	}

	return LoggerContext{
		Logger: logger,
		Writer: parent.Writer,
	}
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
func EventLogger(logCtx LoggerContext, eventName string, eventCfg *EventLoggingConfig) zerolog.Logger {
	logger := logCtx.Logger.With().Str("event", eventName).Logger()
	if eventCfg == nil {
		return logger
	}
	if eventCfg.LogFormat != "" {
		logger = logger.Output(createWriter(eventCfg.LogFormat, logCtx.Writer))
	}
	if eventCfg.Enabled != nil && !*eventCfg.Enabled {
		return logger.Level(zerolog.Disabled)
	}
	if eventCfg.LogLevel != "" {
		return logger.Level(parseLogLevel(eventCfg.LogLevel))
	}
	return logger
}

// createWriter creates a zerolog writer based on the format string.
// It preserves destination by using fallback as the sink.
func createWriter(format string, fallback io.Writer) io.Writer {
	if fallback == nil {
		fallback = os.Stdout
	}
	switch strings.ToLower(format) {
	case "text":
		return zerolog.ConsoleWriter{Out: fallback}
	case "json", "":
		return fallback
	default:
		return fallback
	}
}

// ValidateObservabilityConfig checks that all log_level and log_format values
// in the config (both base and per-event) are recognized. Returns an error on
// the first unrecognized value so operators get fast feedback on typos.
func ValidateObservabilityConfig(cfg *ObservabilityConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validateLogLevel("observability.log_level", cfg.LogLevel); err != nil {
		return err
	}
	if err := validateLogFormat("observability.log_format", cfg.LogFormat); err != nil {
		return err
	}

	events := map[string]*EventLoggingConfig{
		"token_issuance":   cfg.TokenIssuance,
		"token_exchange":   cfg.TokenExchange,
		"authz_check":      cfg.AuthzCheck,
		"config_reload":    cfg.ConfigReload,
		"datasource_cache": cfg.DataSourceCache,
		"lua_datasource":   cfg.LuaDataSource,
		"key_rotation":     cfg.KeyRotation,
		"key_provider":     cfg.KeyProvider,
		"trust_validation": cfg.TrustValidation,
		"jwks_cache":       cfg.JWKSCache,
		"server_lifecycle": cfg.ServerLifecycle,
	}
	for name, ecfg := range events {
		if ecfg == nil {
			continue
		}
		if err := validateLogLevel("observability."+name+".log_level", ecfg.LogLevel); err != nil {
			return err
		}
		if err := validateLogFormat("observability."+name+".log_format", ecfg.LogFormat); err != nil {
			return err
		}
	}

	for i := range cfg.Observers {
		if err := ValidateObservabilityConfig(&cfg.Observers[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateLogLevel(field, value string) error {
	switch strings.ToLower(value) {
	case "", "debug", "info", "warn", "warning", "error":
		return nil
	default:
		return fmt.Errorf("invalid %s %q (valid: debug, info, warn, error)", field, value)
	}
}

func validateLogFormat(field, value string) error {
	switch strings.ToLower(value) {
	case "", "json", "text":
		return nil
	default:
		return fmt.Errorf("invalid %s %q (valid: json, text)", field, value)
	}
}

// parseLogLevel parses a log level string into a zerolog.Level.
func parseLogLevel(levelStr string) zerolog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return zerolog.DebugLevel
	case "info", "":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
