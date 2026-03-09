package keys

// KeyRotationObserver receives key rotation lifecycle events from DualSlotRotatingSigner.
// Implementations are injected at construction time. A nil observer means no events are emitted.
type KeyRotationObserver interface {
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

// KeyProviderObserver receives key provider lifecycle events from AWSKMSKeyProvider.
// A nil observer means no events are emitted.
type KeyProviderObserver interface {
	OldKeyDeletionFailed(keyID string, err error)
}
