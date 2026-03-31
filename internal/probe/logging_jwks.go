package probe

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/server"
)

var _ server.JWKSObserver = (*LoggingJWKSObserver)(nil)

// LoggingJWKSObserver logs JWKS cache lifecycle events via zerolog.
type LoggingJWKSObserver struct {
	server.NoOpJWKSObserver
	logger zerolog.Logger
}

func NewLoggingJWKSObserver(logger zerolog.Logger) *LoggingJWKSObserver {
	return &LoggingJWKSObserver{logger: logger}
}

func (o *LoggingJWKSObserver) CacheRefreshStarted(ctx context.Context) (context.Context, server.CacheRefreshProbe) {
	return ctx, &loggingCacheRefreshProbe{
		logger:    o.logger,
		startTime: time.Now(),
	}
}

type loggingCacheRefreshProbe struct {
	server.NoOpCacheRefreshProbe
	logger    zerolog.Logger
	startTime time.Time
}

func (p *loggingCacheRefreshProbe) InitialCachePopulationFailed(err error) {
	p.logger.Warn().Err(err).Msg("initial cache population failed, will retry")
}

func (p *loggingCacheRefreshProbe) CacheRefreshFailed(err error) {
	p.logger.Warn().Err(err).Msg("background cache refresh failed")
}

func (p *loggingCacheRefreshProbe) KeyConversionFailed(keyID string, err error) {
	p.logger.Warn().Err(err).Str("key_id", keyID).Msg("skipping key: conversion failed")
}

func (p *loggingCacheRefreshProbe) End() {
	p.logger.Debug().
		Dur("duration", time.Since(p.startTime)).
		Msg("cache refresh completed")
}
