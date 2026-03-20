package probe

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/keys"
)

var (
	_ keys.KeyRotationObserver = (*LoggingKeyRotationObserver)(nil)
	_ keys.KeyProviderObserver = (*LoggingKeyProviderObserver)(nil)
)

// LoggingKeyRotationObserver logs key rotation lifecycle events via zerolog.
type LoggingKeyRotationObserver struct {
	logger zerolog.Logger
}

func NewLoggingKeyRotationObserver(logger zerolog.Logger) *LoggingKeyRotationObserver {
	return &LoggingKeyRotationObserver{logger: logger}
}

func (o *LoggingKeyRotationObserver) KeyRotationProbe(_ context.Context) keys.KeyRotationProbe {
	return &loggingKeyRotationProbe{logger: o.logger}
}

type loggingKeyRotationProbe struct {
	logger zerolog.Logger
}

func (p *loggingKeyRotationProbe) RotationCheckFailed(err error) {
	p.logger.Error().Err(err).Msg("key rotation check failed")
}

func (p *loggingKeyRotationProbe) ActiveKeyCacheUpdateFailed(err error) {
	p.logger.Error().Err(err).Msg("active key cache update failed")
}

func (p *loggingKeyRotationProbe) RotationCompleted(slot string) {
	p.logger.Info().Str("slot", slot).Msg("key rotation completed")
}

func (p *loggingKeyRotationProbe) RotationSkippedVersionRace(slot string) {
	p.logger.Info().Str("slot", slot).Msg("another process completed rotation, skipping")
}

func (p *loggingKeyRotationProbe) KeyProviderNotFound(provider, slot string) {
	p.logger.Warn().Str("provider", provider).Str("slot", slot).Msg("key provider not found, skipping")
}

func (p *loggingKeyRotationProbe) KeyHandleFailed(slot string, err error) {
	p.logger.Warn().Err(err).Str("slot", slot).Msg("failed to get key handle")
}

func (p *loggingKeyRotationProbe) PublicKeyFailed(slot string, err error) {
	p.logger.Warn().Err(err).Str("slot", slot).Msg("failed to get public key")
}

func (p *loggingKeyRotationProbe) ThumbprintFailed(slot string, err error) {
	p.logger.Warn().Err(err).Str("slot", slot).Msg("failed to compute thumbprint")
}

func (p *loggingKeyRotationProbe) MetadataFailed(slot string, err error) {
	p.logger.Warn().Err(err).Str("slot", slot).Msg("failed to get key metadata")
}

// LoggingKeyProviderObserver logs key provider lifecycle events via zerolog.
type LoggingKeyProviderObserver struct {
	logger zerolog.Logger
}

func NewLoggingKeyProviderObserver(logger zerolog.Logger) *LoggingKeyProviderObserver {
	return &LoggingKeyProviderObserver{logger: logger}
}

func (o *LoggingKeyProviderObserver) KeyProviderProbe(_ context.Context) keys.KeyProviderProbe {
	return &loggingKeyProviderProbe{logger: o.logger}
}

type loggingKeyProviderProbe struct {
	logger zerolog.Logger
}

func (p *loggingKeyProviderProbe) OldKeyDeletionFailed(keyID string, err error) {
	p.logger.Warn().Err(err).Str("key_id", keyID).Msg("failed to schedule old key for deletion")
}
