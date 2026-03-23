package probe

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/server"
)

var _ server.LifecycleObserver = (*LoggingServerLifecycleObserver)(nil)

// LoggingServerLifecycleObserver logs server lifecycle events via zerolog.
type LoggingServerLifecycleObserver struct {
	logger zerolog.Logger
}

func NewLoggingServerLifecycleObserver(logger zerolog.Logger) *LoggingServerLifecycleObserver {
	return &LoggingServerLifecycleObserver{logger: logger}
}

func (o *LoggingServerLifecycleObserver) ServeStarted(ctx context.Context) (context.Context, server.ServeProbe) {
	return ctx, &loggingServeProbe{
		logger:    o.logger,
		startTime: time.Now(),
	}
}

type loggingServeProbe struct {
	server.NoOpServeProbe
	logger    zerolog.Logger
	startTime time.Time
}

func (p *loggingServeProbe) GRPCServeFailed(err error) {
	p.logger.Error().Err(err).Msg("gRPC server error")
}

func (p *loggingServeProbe) HTTPServeFailed(err error) {
	p.logger.Error().Err(err).Msg("HTTP server error")
}

func (p *loggingServeProbe) End() {
	p.logger.Debug().
		Dur("duration", time.Since(p.startTime)).
		Msg("server stopped")
}
