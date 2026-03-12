package keys

// KeyRotationObserver receives key rotation lifecycle events from DualSlotRotatingSigner.
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
type KeyProviderObserver interface {
	OldKeyDeletionFailed(keyID string, err error)
}

// NoopObserver satisfies both KeyRotationObserver and KeyProviderObserver
// with empty methods. Useful in tests that don't care about observer events.
type NoopObserver struct{}

func (NoopObserver) RotationCheckFailed(error)          {}
func (NoopObserver) ActiveKeyCacheUpdateFailed(error)   {}
func (NoopObserver) RotationCompleted(string)           {}
func (NoopObserver) RotationSkippedVersionRace(string)  {}
func (NoopObserver) KeyProviderNotFound(string, string) {}
func (NoopObserver) KeyHandleFailed(string, error)      {}
func (NoopObserver) PublicKeyFailed(string, error)      {}
func (NoopObserver) ThumbprintFailed(string, error)     {}
func (NoopObserver) MetadataFailed(string, error)       {}
func (NoopObserver) OldKeyDeletionFailed(string, error) {}
