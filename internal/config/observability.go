package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/probe"
	"github.com/project-kessel/parsec/internal/service"
)

// LoggerContext couples a zerolog logger with its destination writer so
// per-event formatting overrides can preserve the original sink.
type LoggerContext struct {
	Logger zerolog.Logger
	Writer io.Writer
}

// NewObserver creates an application observer from configuration.
// This is a convenience wrapper that creates its own logger from cfg.
func NewObserver(cfg *ObservabilityConfig) (service.ApplicationObserver, error) {
	return NewObserverWithLogger(cfg, NewLoggerContext(cfg))
}

// NewObserverWithLogger creates an application observer using the provided logger.
// Use this when you want the observer to share a logger with other components.
func NewObserverWithLogger(cfg *ObservabilityConfig, logCtx LoggerContext) (service.ApplicationObserver, error) {
	if cfg == nil {
		return &service.NoOpApplicationObserver{}, nil
	}

	switch cfg.Type {
	case "logging":
		return probe.NewLoggingObserverWithConfig(probe.LoggingObserverConfig{
			TokenIssuanceLogger: EventLogger(logCtx, "token_issuance", cfg.TokenIssuance),
			TokenExchangeLogger: EventLogger(logCtx, "token_exchange", cfg.TokenExchange),
			AuthzCheckLogger:    EventLogger(logCtx, "authz_check", cfg.AuthzCheck),
		}), nil
	case "noop", "":
		return &service.NoOpApplicationObserver{}, nil
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

// NewLoggerContext creates a structured zerolog logger and the writer used as
// its sink.
func NewLoggerContext(cfg *ObservabilityConfig) LoggerContext {
	if cfg == nil {
		return LoggerContext{
			Logger: zerolog.New(os.Stdout).With().Timestamp().Logger(),
			Writer: os.Stdout,
		}
	}

	defaultLevel := parseLogLevel(cfg.LogLevel)
	writer := createWriter(cfg.LogFormat, os.Stdout)
	return LoggerContext{
		Logger: zerolog.New(writer).With().Timestamp().Logger().Level(defaultLevel),
		Writer: writer,
	}
}

// newCompositeObserver creates a composite observer that delegates to multiple observers
func newCompositeObserver(cfg *ObservabilityConfig, logCtx LoggerContext) (service.ApplicationObserver, error) {
	if len(cfg.Observers) == 0 {
		return nil, fmt.Errorf("composite observer requires at least one sub-observer")
	}

	var observers []service.ApplicationObserver
	for i, subCfg := range cfg.Observers {
		observer, err := NewObserverWithLogger(&subCfg, logCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to create observer %d: %w", i, err)
		}
		observers = append(observers, observer)
	}

	return service.NewCompositeObserver(observers...), nil
}

// EventLogger creates a pre-configured sub-logger for a specific event type.
// The returned logger has the "event" field baked in, its writer set according
// to any per-event log_format override, and its level set according to the
// per-event config. If eventCfg is nil the logger inherits the base settings.
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
