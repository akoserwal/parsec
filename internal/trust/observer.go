package trust

// TrustValidationObserver receives events from FilteredStore during credential validation.
type TrustValidationObserver interface {
	// ValidatorFailed is called when an individual validator fails during
	// multi-validator validation. These intermediate errors are otherwise lost
	// since only the last error is returned to callers.
	ValidatorFailed(validatorName string, credType CredentialType, err error)

	// AllValidatorsFailed is called when all validators fail for a credential type.
	AllValidatorsFailed(credType CredentialType, attempted int, lastErr error)

	// ValidatorFiltered is called when ForActor's policy filter removes a
	// validator from the set available to an actor.
	ValidatorFiltered(validatorName string, actorSubject string)

	// FilterEvaluationFailed is called when the policy filter returns an error
	// for a specific validator.
	FilterEvaluationFailed(validatorName string, err error)
}

// NoopObserver satisfies TrustValidationObserver with empty methods.
// Useful in tests that don't care about observer events.
type NoopObserver struct{}

func (NoopObserver) ValidatorFailed(string, CredentialType, error)  {}
func (NoopObserver) AllValidatorsFailed(CredentialType, int, error) {}
func (NoopObserver) ValidatorFiltered(string, string)               {}
func (NoopObserver) FilterEvaluationFailed(string, error)           {}
