package probe

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/trust"
)

var _ trust.TrustValidationObserver = (*LoggingTrustValidationObserver)(nil)

// LoggingTrustValidationObserver logs trust validation events via zerolog.
type LoggingTrustValidationObserver struct {
	logger zerolog.Logger
}

func NewLoggingTrustValidationObserver(logger zerolog.Logger) *LoggingTrustValidationObserver {
	return &LoggingTrustValidationObserver{logger: logger}
}

func (o *LoggingTrustValidationObserver) TrustValidationProbe(_ context.Context) trust.TrustValidationProbe {
	return &loggingTrustValidationProbe{logger: o.logger}
}

type loggingTrustValidationProbe struct {
	logger zerolog.Logger
}

func (p *loggingTrustValidationProbe) ValidatorFailed(validatorName string, credType trust.CredentialType, err error) {
	p.logger.Debug().
		Err(err).
		Str("validator", validatorName).
		Str("credential_type", string(credType)).
		Msg("validator rejected credential")
}

func (p *loggingTrustValidationProbe) AllValidatorsFailed(credType trust.CredentialType, attempted int, lastErr error) {
	p.logger.Warn().
		Err(lastErr).
		Str("credential_type", string(credType)).
		Int("attempted", attempted).
		Msg("all validators failed for credential type")
}

func (p *loggingTrustValidationProbe) ValidatorFiltered(validatorName string, actorSubject string) {
	p.logger.Debug().
		Str("validator", validatorName).
		Str("actor", actorSubject).
		Msg("validator filtered out for actor")
}

func (p *loggingTrustValidationProbe) FilterEvaluationFailed(validatorName string, err error) {
	p.logger.Error().
		Err(err).
		Str("validator", validatorName).
		Msg("filter evaluation failed")
}
