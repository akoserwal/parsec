package probe

import (
	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/server"
)

var _ server.JWKSObserver = (*LoggingJWKSObserver)(nil)

// LoggingJWKSObserver logs JWKS cache lifecycle events via zerolog.
type LoggingJWKSObserver struct {
	Logger zerolog.Logger
}

func (o *LoggingJWKSObserver) InitialCachePopulationFailed(err error) {
	o.Logger.Warn().Err(err).Msg("initial cache population failed, will retry")
}

func (o *LoggingJWKSObserver) CacheRefreshFailed(err error) {
	o.Logger.Warn().Err(err).Msg("background cache refresh failed")
}

func (o *LoggingJWKSObserver) KeyConversionFailed(keyID string, err error) {
	o.Logger.Warn().Err(err).Str("key_id", keyID).Msg("skipping key: conversion failed")
}
