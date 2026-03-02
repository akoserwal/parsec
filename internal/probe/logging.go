package probe

import (
	"context"
	"os"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/request"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

// loggingObserver creates request-scoped logging probes
type loggingObserver struct {
	service.NoOpApplicationObserver
	logger      zerolog.Logger
	eventLevels map[string]zerolog.Level
}

// LoggingObserverConfig configures the logging observer
type LoggingObserverConfig struct {
	// Logger is the base logger to use.
	Logger zerolog.Logger

	// EventLevels overrides the log level for specific event types.
	// Keys are event names (e.g. "token_issuance"), values are the
	// zerolog.Level to apply. Events not in the map inherit the base
	// logger's level. Use zerolog.Disabled to suppress an event entirely.
	EventLevels map[string]zerolog.Level
}

// NewLoggingObserver creates an application observer that logs all observability events
// using structured logging with zerolog.
func NewLoggingObserver(logger zerolog.Logger) service.ApplicationObserver {
	return NewLoggingObserverWithConfig(LoggingObserverConfig{
		Logger: logger,
	})
}

// NewLoggingObserverWithConfig creates a logging observer with custom configuration
func NewLoggingObserverWithConfig(cfg LoggingObserverConfig) service.ApplicationObserver {
	logger := cfg.Logger
	if logger.GetLevel() == zerolog.Disabled {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	return &loggingObserver{
		logger:      logger,
		eventLevels: cfg.EventLevels,
	}
}

// probeLogger creates a scoped sub-logger for the given event name.
// If a per-event level is configured, the sub-logger's level is set
// accordingly; otherwise it inherits the base logger's level.
func (o *loggingObserver) probeLogger(eventName string) zerolog.Logger {
	l := o.logger.With().Str("event", eventName).Logger()
	if level, ok := o.eventLevels[eventName]; ok {
		l = l.Level(level)
	}
	return l
}

func (o *loggingObserver) TokenIssuanceStarted(
	ctx context.Context,
	subject *trust.Result,
	actor *trust.Result,
	scope string,
	tokenTypes []service.TokenType,
) (context.Context, service.TokenIssuanceProbe) {
	pl := o.probeLogger("token_issuance")

	event := pl.Debug().
		Str("scope", scope).
		Interface("token_types", tokenTypes)

	if subject != nil {
		event = event.
			Str("subject_id", subject.Subject).
			Str("subject_trust_domain", subject.TrustDomain)
	}

	if actor != nil {
		event = event.
			Str("actor_id", actor.Subject).
			Str("actor_trust_domain", actor.TrustDomain)
	}

	event.Msg("Starting token issuance")

	return ctx, &loggingTokenIssuanceProbe{
		logger: pl,
	}
}

// loggingTokenIssuanceProbe is a request-scoped probe that logs events for a single token issuance
type loggingTokenIssuanceProbe struct {
	service.NoOpTokenIssuanceProbe
	logger zerolog.Logger
}

func (p *loggingTokenIssuanceProbe) TokenTypeIssuanceStarted(tokenType service.TokenType) {
	p.logger.Debug().
		Str("token_type", string(tokenType)).
		Msg("Issuing token")
}

func (p *loggingTokenIssuanceProbe) TokenTypeIssuanceSucceeded(tokenType service.TokenType, token *service.Token) {
	event := p.logger.Debug().
		Str("token_type", string(tokenType))

	if token != nil {
		event = event.
			Time("issued_at", token.IssuedAt).
			Time("expires_at", token.ExpiresAt)
	}

	event.Msg("Token issued successfully")
}

func (p *loggingTokenIssuanceProbe) TokenTypeIssuanceFailed(tokenType service.TokenType, err error) {
	p.logger.Error().
		Str("token_type", string(tokenType)).
		Err(err).
		Msg("Token issuance failed")
}

func (p *loggingTokenIssuanceProbe) IssuerNotFound(tokenType service.TokenType, err error) {
	p.logger.Error().
		Str("token_type", string(tokenType)).
		Err(err).
		Msg("No issuer found for token type")
}

func (p *loggingTokenIssuanceProbe) End() {
	p.logger.Debug().Msg("Token issuance completed")
}

// TokenExchangeStarted implements service.TokenExchangeObserver
func (o *loggingObserver) TokenExchangeStarted(
	ctx context.Context,
	grantType string,
	requestedTokenType string,
	audience string,
	scope string,
) (context.Context, service.TokenExchangeProbe) {
	pl := o.probeLogger("token_exchange")

	pl.Debug().
		Str("grant_type", grantType).
		Str("requested_token_type", requestedTokenType).
		Str("audience", audience).
		Str("scope", scope).
		Msg("Starting token exchange")

	return ctx, &loggingTokenExchangeProbe{
		logger: pl,
	}
}

// loggingTokenExchangeProbe is a request-scoped probe that logs token exchange events
type loggingTokenExchangeProbe struct {
	service.NoOpTokenExchangeProbe
	logger zerolog.Logger
}

func (p *loggingTokenExchangeProbe) ActorValidationSucceeded(actor *trust.Result) {
	event := p.logger.Debug()
	if actor != nil {
		event = event.
			Str("actor_id", actor.Subject).
			Str("actor_trust_domain", actor.TrustDomain)
	}
	event.Msg("Actor validation succeeded")
}

func (p *loggingTokenExchangeProbe) ActorValidationFailed(err error) {
	p.logger.Error().
		Err(err).
		Msg("Actor validation failed")
}

func (p *loggingTokenExchangeProbe) RequestContextParsed(attrs *request.RequestAttributes) {
	p.logger.Debug().Msg("Request context parsed")
}

func (p *loggingTokenExchangeProbe) RequestContextParseFailed(err error) {
	p.logger.Error().
		Err(err).
		Msg("Request context parse failed")
}

func (p *loggingTokenExchangeProbe) SubjectTokenValidationSucceeded(subject *trust.Result) {
	event := p.logger.Debug()
	if subject != nil {
		event = event.
			Str("subject_id", subject.Subject).
			Str("subject_trust_domain", subject.TrustDomain)
	}
	event.Msg("Subject token validation succeeded")
}

func (p *loggingTokenExchangeProbe) SubjectTokenValidationFailed(err error) {
	p.logger.Error().
		Err(err).
		Msg("Subject token validation failed")
}

func (p *loggingTokenExchangeProbe) End() {
	p.logger.Debug().Msg("Token exchange completed")
}

// AuthzCheckStarted implements service.AuthzCheckObserver
func (o *loggingObserver) AuthzCheckStarted(
	ctx context.Context,
) (context.Context, service.AuthzCheckProbe) {
	pl := o.probeLogger("authz_check")

	pl.Debug().Msg("Starting authorization check")

	return ctx, &loggingAuthzCheckProbe{
		logger: pl,
	}
}

// loggingAuthzCheckProbe is a request-scoped probe that logs authorization check events
type loggingAuthzCheckProbe struct {
	service.NoOpAuthzCheckProbe
	logger zerolog.Logger
}

func (p *loggingAuthzCheckProbe) RequestAttributesParsed(attrs *request.RequestAttributes) {
	event := p.logger.Debug()
	if attrs != nil {
		event = event.
			Str("method", attrs.Method).
			Str("path", attrs.Path)
	}
	event.Msg("Request attributes parsed")
}

func (p *loggingAuthzCheckProbe) ActorValidationSucceeded(actor *trust.Result) {
	event := p.logger.Debug()
	if actor != nil {
		event = event.
			Str("actor_id", actor.Subject).
			Str("actor_trust_domain", actor.TrustDomain)
	}
	event.Msg("Actor validation succeeded")
}

func (p *loggingAuthzCheckProbe) ActorValidationFailed(err error) {
	p.logger.Error().
		Err(err).
		Msg("Actor validation failed")
}

func (p *loggingAuthzCheckProbe) SubjectCredentialExtracted(cred trust.Credential, headersUsed []string) {
	p.logger.Debug().
		Str("credential_type", string(cred.Type())).
		Msg("Subject credential extracted")
}

func (p *loggingAuthzCheckProbe) SubjectCredentialExtractionFailed(err error) {
	p.logger.Error().
		Err(err).
		Msg("Subject credential extraction failed")
}

func (p *loggingAuthzCheckProbe) SubjectValidationSucceeded(subject *trust.Result) {
	event := p.logger.Debug()
	if subject != nil {
		event = event.
			Str("subject_id", subject.Subject).
			Str("subject_trust_domain", subject.TrustDomain)
	}
	event.Msg("Subject validation succeeded")
}

func (p *loggingAuthzCheckProbe) SubjectValidationFailed(err error) {
	p.logger.Error().
		Err(err).
		Msg("Subject validation failed")
}

func (p *loggingAuthzCheckProbe) End() {
	p.logger.Debug().Msg("Authorization check completed")
}
