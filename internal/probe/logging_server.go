package probe

import (
	"context"

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

func (o *LoggingServerLifecycleObserver) ServerLifecycleProbe(_ context.Context) server.ServerLifecycleProbe {
	return &loggingServerLifecycleProbe{logger: o.logger}
}

type loggingServerLifecycleProbe struct {
	logger zerolog.Logger
}

func (p *loggingServerLifecycleProbe) GRPCServeFailed(err error) {
	p.logger.Error().Err(err).Msg("gRPC server error")
}

func (p *loggingServerLifecycleProbe) HTTPServeFailed(err error) {
	p.logger.Error().Err(err).Msg("HTTP server error")
}
