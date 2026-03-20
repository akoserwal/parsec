package keys

import "context"

// KeyRotationObserver creates probes for key rotation and active-key cache work.
type KeyRotationObserver interface {
	// KeyRotationProbe returns a probe for one rotation tick or cache refresh.
	KeyRotationProbe(ctx context.Context) KeyRotationProbe
}

// KeyRotationProbe receives key rotation lifecycle events for a single scope
// (e.g. one background check or one cache rebuild).
type KeyRotationProbe interface {
	RotationCheckFailed(err error)
	ActiveKeyCacheUpdateFailed(err error)
	RotationCompleted(slot string)
	RotationSkippedVersionRace(slot string)
	KeyProviderNotFound(provider, slot string)
	KeyHandleFailed(slot string, err error)
	PublicKeyFailed(slot string, err error)
	ThumbprintFailed(slot string, err error)
	MetadataFailed(slot string, err error)
}

// KeyProviderObserver creates probes for key provider lifecycle events.
type KeyProviderObserver interface {
	KeyProviderProbe(ctx context.Context) KeyProviderProbe
}

// KeyProviderProbe receives key provider lifecycle events for one operation.
type KeyProviderProbe interface {
	OldKeyDeletionFailed(keyID string, err error)
}

type noopKeyRotationProbe struct{}

func (noopKeyRotationProbe) RotationCheckFailed(error)          {}
func (noopKeyRotationProbe) ActiveKeyCacheUpdateFailed(error)   {}
func (noopKeyRotationProbe) RotationCompleted(string)           {}
func (noopKeyRotationProbe) RotationSkippedVersionRace(string)  {}
func (noopKeyRotationProbe) KeyProviderNotFound(string, string) {}
func (noopKeyRotationProbe) KeyHandleFailed(string, error)      {}
func (noopKeyRotationProbe) PublicKeyFailed(string, error)      {}
func (noopKeyRotationProbe) ThumbprintFailed(string, error)     {}
func (noopKeyRotationProbe) MetadataFailed(string, error)       {}

type noopKeyProviderProbe struct{}

func (noopKeyProviderProbe) OldKeyDeletionFailed(string, error) {}

// NoopObserver satisfies both KeyRotationObserver and KeyProviderObserver
// with empty probes. Useful in tests that don't care about observer events.
type NoopObserver struct{}

func (NoopObserver) KeyRotationProbe(context.Context) KeyRotationProbe {
	return noopKeyRotationProbe{}
}

func (NoopObserver) KeyProviderProbe(context.Context) KeyProviderProbe {
	return noopKeyProviderProbe{}
}
