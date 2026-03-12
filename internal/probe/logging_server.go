package probe

import (
	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/server"
)

var _ server.ServerLifecycleObserver = (*LoggingServerLifecycleObserver)(nil)

// LoggingServerLifecycleObserver logs server lifecycle events via zerolog.
type LoggingServerLifecycleObserver struct {
	logger zerolog.Logger
}

func NewLoggingServerLifecycleObserver(logger zerolog.Logger) *LoggingServerLifecycleObserver {
	return &LoggingServerLifecycleObserver{logger: logger}
}

func (o *LoggingServerLifecycleObserver) GRPCServeFailed(err error) {
	o.logger.Error().Err(err).Msg("gRPC server error")
}

func (o *LoggingServerLifecycleObserver) HTTPServeFailed(err error) {
	o.logger.Error().Err(err).Msg("HTTP server error")
}
