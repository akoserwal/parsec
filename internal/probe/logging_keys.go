package probe

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/keys"
)

var (
	_ keys.RotationObserver = (*LoggingKeyRotationObserver)(nil)
	_ keys.ProviderObserver = (*LoggingKeyProviderObserver)(nil)
)

// LoggingKeyRotationObserver logs key rotation lifecycle events via zerolog.
type LoggingKeyRotationObserver struct {
	keys.NoOpRotationObserver
	logger zerolog.Logger
}

func NewLoggingKeyRotationObserver(logger zerolog.Logger) *LoggingKeyRotationObserver {
	return &LoggingKeyRotationObserver{logger: logger}
}

func (o *LoggingKeyRotationObserver) RotationCheckStarted(ctx context.Context) (context.Context, keys.RotationCheckProbe) {
	return ctx, &loggingRotationCheckProbe{
		logger:    o.logger,
		startTime: time.Now(),
	}
}

func (o *LoggingKeyRotationObserver) KeyCacheUpdateStarted(ctx context.Context) (context.Context, keys.KeyCacheUpdateProbe) {
	return ctx, &loggingKeyCacheUpdateProbe{
		logger:    o.logger,
		startTime: time.Now(),
	}
}

type loggingRotationCheckProbe struct {
	keys.NoOpRotationCheckProbe
	logger    zerolog.Logger
	startTime time.Time
}

func (p *loggingRotationCheckProbe) RotationCheckFailed(err error) {
	p.logger.Error().Err(err).Msg("key rotation check failed")
}

func (p *loggingRotationCheckProbe) RotationCompleted(slot string) {
	p.logger.Info().Str("slot", slot).Msg("key rotation completed")
}

func (p *loggingRotationCheckProbe) RotationSkippedVersionRace(slot string) {
	p.logger.Info().Str("slot", slot).Msg("another process completed rotation, skipping")
}

func (p *loggingRotationCheckProbe) End() {
	p.logger.Debug().
		Dur("duration", time.Since(p.startTime)).
		Msg("rotation check completed")
}

type loggingKeyCacheUpdateProbe struct {
	keys.NoOpKeyCacheUpdateProbe
	logger    zerolog.Logger
	startTime time.Time
}

func (p *loggingKeyCacheUpdateProbe) KeyCacheUpdateFailed(err error) {
	p.logger.Error().Err(err).Msg("active key cache update failed")
}

func (p *loggingKeyCacheUpdateProbe) KeyProviderNotFound(provider, slot string) {
	p.logger.Warn().Str("provider", provider).Str("slot", slot).Msg("key provider not found, skipping")
}

func (p *loggingKeyCacheUpdateProbe) KeyHandleFailed(slot string, err error) {
	p.logger.Warn().Err(err).Str("slot", slot).Msg("failed to get key handle")
}

func (p *loggingKeyCacheUpdateProbe) PublicKeyFailed(slot string, err error) {
	p.logger.Warn().Err(err).Str("slot", slot).Msg("failed to get public key")
}

func (p *loggingKeyCacheUpdateProbe) ThumbprintFailed(slot string, err error) {
	p.logger.Warn().Err(err).Str("slot", slot).Msg("failed to compute thumbprint")
}

func (p *loggingKeyCacheUpdateProbe) MetadataFailed(slot string, err error) {
	p.logger.Warn().Err(err).Str("slot", slot).Msg("failed to get key metadata")
}

func (p *loggingKeyCacheUpdateProbe) End() {
	p.logger.Debug().
		Dur("duration", time.Since(p.startTime)).
		Msg("key cache update completed")
}

// LoggingKeyProviderObserver logs key provider lifecycle events via zerolog.
type LoggingKeyProviderObserver struct {
	keys.NoOpProviderObserver
	logger zerolog.Logger
}

func NewLoggingKeyProviderObserver(logger zerolog.Logger) *LoggingKeyProviderObserver {
	return &LoggingKeyProviderObserver{logger: logger}
}

func (o *LoggingKeyProviderObserver) KeyProvisionStarted(ctx context.Context) (context.Context, keys.KeyProvisionProbe) {
	return ctx, &loggingKeyProvisionProbe{
		logger:    o.logger,
		startTime: time.Now(),
	}
}

type loggingKeyProvisionProbe struct {
	keys.NoOpKeyProvisionProbe
	logger    zerolog.Logger
	startTime time.Time
}

func (p *loggingKeyProvisionProbe) OldKeyDeletionFailed(keyID string, err error) {
	p.logger.Warn().Err(err).Str("key_id", keyID).Msg("failed to schedule old key for deletion")
}

func (p *loggingKeyProvisionProbe) End() {
	p.logger.Debug().
		Dur("duration", time.Since(p.startTime)).
		Msg("key provision completed")
}
