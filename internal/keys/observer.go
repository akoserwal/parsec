package keys

import "context"

// RotationObserver is called at key points during key rotation operations.
// Implementations should embed NoOpRotationObserver for forward compatibility
// with new methods added to this interface.
type RotationObserver interface {
	// RotationCheckStarted is called when a rotation check begins.
	// Returns a potentially modified context and a probe to track the operation.
	RotationCheckStarted(ctx context.Context) (context.Context, RotationCheckProbe)
}

// RotationCheckProbe tracks a single rotation check invocation.
// Implementations should embed NoOpRotationCheckProbe for forward compatibility.
type RotationCheckProbe interface {
	RotationCheckFailed(err error)
	ActiveKeyCacheUpdateFailed(err error)
	RotationCompleted(slot string)
	RotationSkippedVersionRace(slot string)
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
// Embed this in concrete probe types for forward compatibility.
type NoOpRotationCheckProbe struct{}

func (NoOpRotationCheckProbe) RotationCheckFailed(error)          {}
func (NoOpRotationCheckProbe) ActiveKeyCacheUpdateFailed(error)   {}
func (NoOpRotationCheckProbe) RotationCompleted(string)           {}
func (NoOpRotationCheckProbe) RotationSkippedVersionRace(string)  {}
func (NoOpRotationCheckProbe) KeyProviderNotFound(string, string) {}
func (NoOpRotationCheckProbe) KeyHandleFailed(string, error)      {}
func (NoOpRotationCheckProbe) PublicKeyFailed(string, error)      {}
func (NoOpRotationCheckProbe) ThumbprintFailed(string, error)     {}
func (NoOpRotationCheckProbe) MetadataFailed(string, error)       {}
func (NoOpRotationCheckProbe) End()                               {}

// NoOpKeyProvisionProbe is a no-op implementation of KeyProvisionProbe.
// Embed this in concrete probe types for forward compatibility.
type NoOpKeyProvisionProbe struct{}

func (NoOpKeyProvisionProbe) OldKeyDeletionFailed(string, error) {}
func (NoOpKeyProvisionProbe) End()                               {}

// NoOpRotationObserver is a no-op implementation of RotationObserver.
// Embed this in concrete observer types for forward compatibility.
type NoOpRotationObserver struct{}

func (NoOpRotationObserver) RotationCheckStarted(ctx context.Context) (context.Context, RotationCheckProbe) {
	return ctx, NoOpRotationCheckProbe{}
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
