package trust

import "context"

// ValidationObserver is called at key points during trust validation operations.
// Implementations should embed NoOpValidationObserver for forward compatibility
// with new methods added to this interface.
type ValidationObserver interface {
	// ValidationStarted is called when a trust validation operation begins.
	// Returns a potentially modified context and a probe to track the operation.
	ValidationStarted(ctx context.Context) (context.Context, ValidationProbe)
}

// ValidationProbe tracks a single trust validation invocation.
// Implementations should embed NoOpValidationProbe for forward compatibility.
type ValidationProbe interface {
	ValidatorFailed(validatorName string, credType CredentialType, err error)
	AllValidatorsFailed(credType CredentialType, attempted int, lastErr error)
	ValidatorFiltered(validatorName string, actorSubject string)
	FilterEvaluationFailed(validatorName string, err error)
	End()
}

// TrustObserver is the per-package aggregate for all trust observer interfaces.
type TrustObserver interface {
	ValidationObserver
}

// --- NoOp implementations ---

// NoOpValidationProbe is a no-op implementation of ValidationProbe.
// Embed this in concrete probe types for forward compatibility.
type NoOpValidationProbe struct{}

func (NoOpValidationProbe) ValidatorFailed(string, CredentialType, error)  {}
func (NoOpValidationProbe) AllValidatorsFailed(CredentialType, int, error) {}
func (NoOpValidationProbe) ValidatorFiltered(string, string)               {}
func (NoOpValidationProbe) FilterEvaluationFailed(string, error)           {}
func (NoOpValidationProbe) End()                                           {}

// NoOpObserver satisfies ValidationObserver with empty probes.
// Useful in tests that don't care about observer events.
type NoOpObserver struct{}

func (NoOpObserver) ValidationStarted(ctx context.Context) (context.Context, ValidationProbe) {
	return ctx, NoOpValidationProbe{}
}

var _ TrustObserver = NoOpObserver{}
