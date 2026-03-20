package probe

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/server"
)

var _ server.JWKSObserver = (*LoggingJWKSObserver)(nil)

// LoggingJWKSObserver logs JWKS cache lifecycle events via zerolog.
type LoggingJWKSObserver struct {
	logger zerolog.Logger
}

func NewLoggingJWKSObserver(logger zerolog.Logger) *LoggingJWKSObserver {
	return &LoggingJWKSObserver{logger: logger}
}

func (o *LoggingJWKSObserver) JWKSCacheProbe(_ context.Context) server.JWKSCacheProbe {
	return &loggingJWKSCacheProbe{logger: o.logger}
}

type loggingJWKSCacheProbe struct {
	logger zerolog.Logger
}

func (p *loggingJWKSCacheProbe) InitialCachePopulationFailed(err error) {
	p.logger.Warn().Err(err).Msg("initial cache population failed, will retry")
}

func (p *loggingJWKSCacheProbe) CacheRefreshFailed(err error) {
	p.logger.Warn().Err(err).Msg("background cache refresh failed")
}

func (p *loggingJWKSCacheProbe) KeyConversionFailed(keyID string, err error) {
	p.logger.Warn().Err(err).Str("key_id", keyID).Msg("skipping key: conversion failed")
}
