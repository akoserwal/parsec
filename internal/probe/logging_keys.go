package probe

import (
	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/keys"
)

// Compile-time interface checks.
var (
	_ keys.KeyRotationObserver = (*LoggingKeyRotationObserver)(nil)
	_ keys.KeyProviderObserver = (*LoggingKeyProviderObserver)(nil)
)

// LoggingKeyRotationObserver logs key rotation lifecycle events via zerolog.
type LoggingKeyRotationObserver struct {
	Logger zerolog.Logger
}

func (o *LoggingKeyRotationObserver) RotationCheckFailed(err error) {
	o.Logger.Error().Err(err).Msg("key rotation check failed")
}

func (o *LoggingKeyRotationObserver) ActiveKeyCacheUpdateFailed(err error) {
	o.Logger.Error().Err(err).Msg("active key cache update failed")
}

func (o *LoggingKeyRotationObserver) RotationCompleted(slot string) {
	o.Logger.Info().Str("slot", slot).Msg("key rotation completed")
}

func (o *LoggingKeyRotationObserver) RotationSkippedVersionRace(slot string) {
	o.Logger.Info().Str("slot", slot).Msg("another process completed rotation, skipping")
}

func (o *LoggingKeyRotationObserver) KeyProviderNotFound(provider, slot string) {
	o.Logger.Warn().Str("provider", provider).Str("slot", slot).Msg("key provider not found, skipping")
}

func (o *LoggingKeyRotationObserver) KeyHandleFailed(slot string, err error) {
	o.Logger.Warn().Err(err).Str("slot", slot).Msg("failed to get key handle")
}

func (o *LoggingKeyRotationObserver) PublicKeyFailed(slot string, err error) {
	o.Logger.Warn().Err(err).Str("slot", slot).Msg("failed to get public key")
}

func (o *LoggingKeyRotationObserver) ThumbprintFailed(slot string, err error) {
	o.Logger.Warn().Err(err).Str("slot", slot).Msg("failed to compute thumbprint")
}

func (o *LoggingKeyRotationObserver) MetadataFailed(slot string, err error) {
	o.Logger.Warn().Err(err).Str("slot", slot).Msg("failed to get key metadata")
}

// LoggingKeyProviderObserver logs key provider lifecycle events via zerolog.
type LoggingKeyProviderObserver struct {
	Logger zerolog.Logger
}

func (o *LoggingKeyProviderObserver) OldKeyDeletionFailed(keyID string, err error) {
	o.Logger.Warn().Err(err).Str("key_id", keyID).Msg("failed to schedule old key for deletion")
}
