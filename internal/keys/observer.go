package keys

import "context"

// RotationObserver is called at key points during key rotation operations.
// Implementations should embed NoOpRotationObserver for forward compatibility
// with new methods added to this interface.
type RotationObserver interface {
	// RotationCheckStarted is called when a rotation check begins.
	RotationCheckStarted(ctx context.Context) (context.Context, RotationCheckProbe)
	// KeyCacheUpdateStarted is called when the active key cache is being rebuilt.
	KeyCacheUpdateStarted(ctx context.Context) (context.Context, KeyCacheUpdateProbe)
}

// RotationCheckProbe tracks a single rotation check invocation.
// Implementations should embed NoOpRotationCheckProbe for forward compatibility.
type RotationCheckProbe interface {
	RotationCheckFailed(err error)
	RotationCompleted(slot string)
	RotationSkippedVersionRace(slot string)
	End()
}

// KeyCacheUpdateProbe tracks a single active-key-cache rebuild.
// Implementations should embed NoOpKeyCacheUpdateProbe for forward compatibility.
type KeyCacheUpdateProbe interface {
	KeyCacheUpdateFailed(err error)
	KeyProviderNotFound(provider, slot string)
	KeyHandleFailed(slot string, err error)
	PublicKeyFailed(slot string, err error)
	ThumbprintFailed(slot string, err error)
	MetadataFailed(slot string, err error)
	End()
}

// ProviderObserver is called at key points during key provisioning operations.
// Implementations should embed NoOpProviderObserver for forward compatibility
// with new methods added to this interface.
type ProviderObserver interface {
	// KeyProvisionStarted is called when a key provision operation begins.
	// Returns a potentially modified context and a probe to track the operation.
	KeyProvisionStarted(ctx context.Context) (context.Context, KeyProvisionProbe)
}

// KeyProvisionProbe tracks a single key provisioning invocation.
// Implementations should embed NoOpKeyProvisionProbe for forward compatibility.
type KeyProvisionProbe interface {
	OldKeyDeletionFailed(keyID string, err error)
	End()
}

// KeysObserver is the per-package aggregate for all keys observer interfaces.
type KeysObserver interface {
	RotationObserver
	ProviderObserver
}

// --- NoOp implementations ---

// NoOpRotationCheckProbe is a no-op implementation of RotationCheckProbe.
type NoOpRotationCheckProbe struct{}

func (NoOpRotationCheckProbe) RotationCheckFailed(error)         {}
func (NoOpRotationCheckProbe) RotationCompleted(string)          {}
func (NoOpRotationCheckProbe) RotationSkippedVersionRace(string) {}
func (NoOpRotationCheckProbe) End()                              {}

// NoOpKeyCacheUpdateProbe is a no-op implementation of KeyCacheUpdateProbe.
type NoOpKeyCacheUpdateProbe struct{}

func (NoOpKeyCacheUpdateProbe) KeyCacheUpdateFailed(error)         {}
func (NoOpKeyCacheUpdateProbe) KeyProviderNotFound(string, string) {}
func (NoOpKeyCacheUpdateProbe) KeyHandleFailed(string, error)      {}
func (NoOpKeyCacheUpdateProbe) PublicKeyFailed(string, error)      {}
func (NoOpKeyCacheUpdateProbe) ThumbprintFailed(string, error)     {}
func (NoOpKeyCacheUpdateProbe) MetadataFailed(string, error)       {}
func (NoOpKeyCacheUpdateProbe) End()                               {}

// NoOpKeyProvisionProbe is a no-op implementation of KeyProvisionProbe.
type NoOpKeyProvisionProbe struct{}

func (NoOpKeyProvisionProbe) OldKeyDeletionFailed(string, error) {}
func (NoOpKeyProvisionProbe) End()                               {}

// NoOpRotationObserver is a no-op implementation of RotationObserver.
type NoOpRotationObserver struct{}

func (NoOpRotationObserver) RotationCheckStarted(ctx context.Context) (context.Context, RotationCheckProbe) {
	return ctx, NoOpRotationCheckProbe{}
}

func (NoOpRotationObserver) KeyCacheUpdateStarted(ctx context.Context) (context.Context, KeyCacheUpdateProbe) {
	return ctx, NoOpKeyCacheUpdateProbe{}
}

// NoOpProviderObserver is a no-op implementation of ProviderObserver.
// Embed this in concrete observer types for forward compatibility.
type NoOpProviderObserver struct{}

func (NoOpProviderObserver) KeyProvisionStarted(ctx context.Context) (context.Context, KeyProvisionProbe) {
	return ctx, NoOpKeyProvisionProbe{}
}

// NoOpObserver satisfies both RotationObserver and ProviderObserver
// with empty probes. Useful in tests that don't care about observer events.
type NoOpObserver struct {
	NoOpRotationObserver
	NoOpProviderObserver
}

var _ KeysObserver = NoOpObserver{}
