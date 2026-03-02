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

// NewObserver creates an application observer from configuration.
// This is a convenience wrapper that creates its own logger from cfg.
func NewObserver(cfg *ObservabilityConfig) (service.ApplicationObserver, error) {
	return NewObserverWithLogger(cfg, NewLogger(cfg))
}

// NewObserverWithLogger creates an application observer using the provided logger.
// Use this when you want the observer to share a logger with other components.
func NewObserverWithLogger(cfg *ObservabilityConfig, logger zerolog.Logger) (service.ApplicationObserver, error) {
	if cfg == nil {
		return &service.NoOpApplicationObserver{}, nil
	}

	switch cfg.Type {
	case "logging":
		return probe.NewLoggingObserverWithConfig(probe.LoggingObserverConfig{
			Logger:      logger,
			EventLevels: buildEventLevels(cfg, logger.GetLevel()),
		}), nil
	case "noop", "":
		return &service.NoOpApplicationObserver{}, nil
	case "composite":
		return newCompositeObserver(cfg)
	default:
		return nil, fmt.Errorf("unknown observability type: %s (supported: logging, noop, composite)", cfg.Type)
	}
}

// NewLogger creates a structured zerolog logger from the observability configuration.
func NewLogger(cfg *ObservabilityConfig) zerolog.Logger {
	if cfg == nil {
		return zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	defaultLevel := parseLogLevel(cfg.LogLevel)
	writer := createWriter(cfg.LogFormat)
	return zerolog.New(writer).With().Timestamp().Logger().Level(defaultLevel)
}

// newCompositeObserver creates a composite observer that delegates to multiple observers
func newCompositeObserver(cfg *ObservabilityConfig) (service.ApplicationObserver, error) {
	if len(cfg.Observers) == 0 {
		return nil, fmt.Errorf("composite observer requires at least one sub-observer")
	}

	var observers []service.ApplicationObserver
	for i, subCfg := range cfg.Observers {
		observer, err := NewObserver(&subCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create observer %d: %w", i, err)
		}
		observers = append(observers, observer)
	}

	return service.NewCompositeObserver(observers...), nil
}

// buildEventLevels creates a map of event-specific log levels from config.
// Only events with explicit overrides are included; events inheriting the
// base level are omitted so the observer uses its logger's default.
func buildEventLevels(cfg *ObservabilityConfig, baseLevel zerolog.Level) map[string]zerolog.Level {
	events := []string{"token_issuance", "token_exchange", "authz_check"}
	levels := make(map[string]zerolog.Level)
	for _, name := range events {
		level := EventLevel(cfg, name, baseLevel)
		if level != baseLevel {
			levels[name] = level
		}
	}
	return levels
}

// EventLevel returns the configured level for a given event name, or the
// provided fallback if no event-specific level is configured.
func EventLevel(cfg *ObservabilityConfig, eventName string, fallback zerolog.Level) zerolog.Level {
	if cfg == nil {
		return fallback
	}

	var ec *EventLoggingConfig
	switch eventName {
	case "token_issuance":
		ec = cfg.TokenIssuance
	case "token_exchange":
		ec = cfg.TokenExchange
	case "authz_check":
		ec = cfg.AuthzCheck
	}

	if ec == nil {
		return fallback
	}
	if ec.Enabled != nil && !*ec.Enabled {
		return zerolog.Disabled
	}
	if ec.LogLevel != "" {
		return parseLogLevel(ec.LogLevel)
	}
	return fallback
}

// createWriter creates a zerolog writer based on the format string.
func createWriter(format string) io.Writer {
	switch strings.ToLower(format) {
	case "text":
		return zerolog.ConsoleWriter{Out: os.Stdout}
	case "json", "":
		return os.Stdout
	default:
		return os.Stdout
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
