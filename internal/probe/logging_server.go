package probe

import (
	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/server"
)

var _ server.ServerLifecycleObserver = (*LoggingServerLifecycleObserver)(nil)

// LoggingServerLifecycleObserver logs server lifecycle events via zerolog.
type LoggingServerLifecycleObserver struct {
	Logger zerolog.Logger
}

func (o *LoggingServerLifecycleObserver) GRPCServeFailed(err error) {
	o.Logger.Error().Err(err).Msg("gRPC server error")
}

func (o *LoggingServerLifecycleObserver) HTTPServeFailed(err error) {
	o.Logger.Error().Err(err).Msg("HTTP server error")
}
