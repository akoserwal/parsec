package trust

import "context"

// TrustValidationObserver creates probes for credential validation and filtering.
type TrustValidationObserver interface {
	// TrustValidationProbe returns a probe for one Validate or ForActor operation.
	TrustValidationProbe(ctx context.Context) TrustValidationProbe
}

// TrustValidationProbe receives trust validation events without passing context
// to each method.
type TrustValidationProbe interface {
	ValidatorFailed(validatorName string, credType CredentialType, err error)
	AllValidatorsFailed(credType CredentialType, attempted int, lastErr error)
	ValidatorFiltered(validatorName string, actorSubject string)
	FilterEvaluationFailed(validatorName string, err error)
}

type noopTrustValidationProbe struct{}

func (noopTrustValidationProbe) ValidatorFailed(string, CredentialType, error)  {}
func (noopTrustValidationProbe) AllValidatorsFailed(CredentialType, int, error) {}
func (noopTrustValidationProbe) ValidatorFiltered(string, string)               {}
func (noopTrustValidationProbe) FilterEvaluationFailed(string, error)           {}

// NoopObserver satisfies TrustValidationObserver with empty probes.
// Useful in tests that don't care about observer events.
type NoopObserver struct{}

func (NoopObserver) TrustValidationProbe(context.Context) TrustValidationProbe {
	return noopTrustValidationProbe{}
}
