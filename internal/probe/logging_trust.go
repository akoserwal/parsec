package probe

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/trust"
)

var (
	_ trust.ValidationObserver = (*LoggingTrustValidationObserver)(nil)
	_ trust.TrustObserver      = (*LoggingTrustValidationObserver)(nil)
)

// LoggingTrustValidationObserver logs trust validation events via zerolog.
type LoggingTrustValidationObserver struct {
	trust.NoOpValidationObserver
	logger zerolog.Logger
}

func NewLoggingTrustValidationObserver(logger zerolog.Logger) *LoggingTrustValidationObserver {
	return &LoggingTrustValidationObserver{logger: logger}
}

func (o *LoggingTrustValidationObserver) ValidationStarted(ctx context.Context) (context.Context, trust.ValidationProbe) {
	return ctx, &loggingValidationProbe{
		logger:    o.logger,
		startTime: time.Now(),
	}
}

type loggingValidationProbe struct {
	trust.NoOpValidationProbe
	logger    zerolog.Logger
	startTime time.Time
}

func (p *loggingValidationProbe) ValidatorFailed(validatorName string, credType trust.CredentialType, err error) {
	p.logger.Debug().
		Err(err).
		Str("validator", validatorName).
		Str("credential_type", string(credType)).
		Msg("validator rejected credential")
}

func (p *loggingValidationProbe) AllValidatorsFailed(credType trust.CredentialType, attempted int, lastErr error) {
	p.logger.Warn().
		Err(lastErr).
		Str("credential_type", string(credType)).
		Int("attempted", attempted).
		Msg("all validators failed for credential type")
}

func (p *loggingValidationProbe) ValidatorFiltered(validatorName string, actorSubject string) {
	p.logger.Debug().
		Str("validator", validatorName).
		Str("actor", actorSubject).
		Msg("validator filtered out for actor")
}

func (p *loggingValidationProbe) FilterEvaluationFailed(validatorName string, err error) {
	p.logger.Error().
		Err(err).
		Str("validator", validatorName).
		Msg("filter evaluation failed")
}

func (p *loggingValidationProbe) End() {
	p.logger.Debug().
		Dur("duration", time.Since(p.startTime)).
		Msg("trust validation completed")
}
