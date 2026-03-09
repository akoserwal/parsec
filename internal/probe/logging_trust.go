package probe

import (
	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/trust"
)

// Compile-time interface check.
var _ trust.TrustValidationObserver = (*LoggingTrustValidationObserver)(nil)

// LoggingTrustValidationObserver logs trust validation events via zerolog.
type LoggingTrustValidationObserver struct {
	Logger zerolog.Logger
}

func (o *LoggingTrustValidationObserver) ValidatorFailed(validatorName string, credType trust.CredentialType, err error) {
	o.Logger.Debug().
		Err(err).
		Str("validator", validatorName).
		Str("credential_type", string(credType)).
		Msg("validator rejected credential")
}

func (o *LoggingTrustValidationObserver) AllValidatorsFailed(credType trust.CredentialType, attempted int, lastErr error) {
	o.Logger.Warn().
		Err(lastErr).
		Str("credential_type", string(credType)).
		Int("attempted", attempted).
		Msg("all validators failed for credential type")
}

func (o *LoggingTrustValidationObserver) ValidatorFiltered(validatorName string, actorSubject string) {
	o.Logger.Debug().
		Str("validator", validatorName).
		Str("actor", actorSubject).
		Msg("validator filtered out for actor")
}

func (o *LoggingTrustValidationObserver) FilterEvaluationFailed(validatorName string, err error) {
	o.Logger.Error().
		Err(err).
		Str("validator", validatorName).
		Msg("filter evaluation failed")
}
