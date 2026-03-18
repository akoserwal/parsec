package probe

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

// metricsObserver creates request-scoped metrics probes.
type metricsObserver struct {
	service.NoOpApplicationObserver

	// Token issuance instruments
	tokenIssuanceTotal    metric.Int64Counter
	tokenIssuanceDuration metric.Float64Histogram

	// Token exchange instruments
	tokenExchangeTotal              metric.Int64Counter
	tokenExchangeDuration           metric.Float64Histogram
	tokenExchangeValidationFailures metric.Int64Counter

	// Authz check instruments
	authzCheckTotal              metric.Int64Counter
	authzCheckDuration           metric.Float64Histogram
	authzCheckValidationFailures metric.Int64Counter
}

// NewMetricsObserver creates an application observer that records OTel metrics
// for token issuance, token exchange, and authorization check operations.
func NewMetricsObserver(meter metric.Meter) (service.ApplicationObserver, error) {
	o := &metricsObserver{}
	var err error

	if o.tokenIssuanceTotal, err = meter.Int64Counter("parsec_token_issuance_total",
		metric.WithDescription("Total token issuance attempts"),
	); err != nil {
		return nil, err
	}
	if o.tokenIssuanceDuration, err = meter.Float64Histogram("parsec_token_issuance_duration_seconds",
		metric.WithDescription("Duration of token issuance operations"),
		metric.WithUnit("s"),
	); err != nil {
		return nil, err
	}

	if o.tokenExchangeTotal, err = meter.Int64Counter("parsec_token_exchange_total",
		metric.WithDescription("Total token exchange requests"),
	); err != nil {
		return nil, err
	}
	if o.tokenExchangeDuration, err = meter.Float64Histogram("parsec_token_exchange_duration_seconds",
		metric.WithDescription("Duration of token exchange operations"),
		metric.WithUnit("s"),
	); err != nil {
		return nil, err
	}
	if o.tokenExchangeValidationFailures, err = meter.Int64Counter("parsec_token_exchange_validation_failures_total",
		metric.WithDescription("Token exchange validation failures by stage"),
	); err != nil {
		return nil, err
	}

	if o.authzCheckTotal, err = meter.Int64Counter("parsec_authz_check_total",
		metric.WithDescription("Total authorization check requests"),
	); err != nil {
		return nil, err
	}
	if o.authzCheckDuration, err = meter.Float64Histogram("parsec_authz_check_duration_seconds",
		metric.WithDescription("Duration of authorization check operations"),
		metric.WithUnit("s"),
	); err != nil {
		return nil, err
	}
	if o.authzCheckValidationFailures, err = meter.Int64Counter("parsec_authz_check_validation_failures_total",
		metric.WithDescription("Authorization check validation failures by stage"),
	); err != nil {
		return nil, err
	}

	return o, nil
}

// --- Token Issuance ---

func (o *metricsObserver) TokenIssuanceStarted(
	ctx context.Context,
	_ *trust.Result,
	_ *trust.Result,
	_ string,
	_ []service.TokenType,
) (context.Context, service.TokenIssuanceProbe) {
	return ctx, &metricsTokenIssuanceProbe{
		counter:  o.tokenIssuanceTotal,
		duration: o.tokenIssuanceDuration,
		start:    time.Now(),
	}
}

type metricsTokenIssuanceProbe struct {
	service.NoOpTokenIssuanceProbe
	counter  metric.Int64Counter
	duration metric.Float64Histogram
	start    time.Time
	mu       sync.Mutex
	// Track per-token-type outcomes so End() can record overall duration.
	// The last token type seen is used as the duration label.
	lastTokenType string
	failed        bool
}

func (p *metricsTokenIssuanceProbe) TokenTypeIssuanceSucceeded(tokenType service.TokenType, _ *service.Token) {
	p.mu.Lock()
	p.lastTokenType = string(tokenType)
	p.mu.Unlock()
	p.counter.Add(context.Background(), 1,
		metric.WithAttributes(
			attribute.String("token_type", string(tokenType)),
			attribute.String("status", "success"),
		),
	)
}

func (p *metricsTokenIssuanceProbe) TokenTypeIssuanceFailed(tokenType service.TokenType, _ error) {
	p.mu.Lock()
	p.lastTokenType = string(tokenType)
	p.failed = true
	p.mu.Unlock()
	p.counter.Add(context.Background(), 1,
		metric.WithAttributes(
			attribute.String("token_type", string(tokenType)),
			attribute.String("status", "failure"),
		),
	)
}

func (p *metricsTokenIssuanceProbe) IssuerNotFound(tokenType service.TokenType, _ error) {
	p.mu.Lock()
	p.lastTokenType = string(tokenType)
	p.failed = true
	p.mu.Unlock()
	p.counter.Add(context.Background(), 1,
		metric.WithAttributes(
			attribute.String("token_type", string(tokenType)),
			attribute.String("status", "issuer_not_found"),
		),
	)
}

func (p *metricsTokenIssuanceProbe) End() {
	p.mu.Lock()
	tt := p.lastTokenType
	p.mu.Unlock()
	elapsed := time.Since(p.start).Seconds()
	p.duration.Record(context.Background(), elapsed,
		metric.WithAttributes(attribute.String("token_type", tt)),
	)
}

// --- Token Exchange ---

func (o *metricsObserver) TokenExchangeStarted(
	ctx context.Context,
	_ string,
	_ string,
	_ string,
	_ string,
) (context.Context, service.TokenExchangeProbe) {
	return ctx, &metricsTokenExchangeProbe{
		total:              o.tokenExchangeTotal,
		duration:           o.tokenExchangeDuration,
		validationFailures: o.tokenExchangeValidationFailures,
		start:              time.Now(),
	}
}

type metricsTokenExchangeProbe struct {
	service.NoOpTokenExchangeProbe
	total              metric.Int64Counter
	duration           metric.Float64Histogram
	validationFailures metric.Int64Counter
	start              time.Time
	mu                 sync.Mutex
	failed             bool
}

func (p *metricsTokenExchangeProbe) ActorValidationFailed(_ error) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
	p.validationFailures.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("stage", "actor")),
	)
}

func (p *metricsTokenExchangeProbe) SubjectTokenValidationFailed(_ error) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
	p.validationFailures.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("stage", "subject")),
	)
}

func (p *metricsTokenExchangeProbe) RequestContextParseFailed(_ error) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
	p.validationFailures.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("stage", "request_context")),
	)
}

func (p *metricsTokenExchangeProbe) End() {
	p.mu.Lock()
	failed := p.failed
	p.mu.Unlock()

	status := "success"
	if failed {
		status = "failure"
	}
	p.total.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("status", status)),
	)
	p.duration.Record(context.Background(), time.Since(p.start).Seconds(),
		metric.WithAttributes(attribute.String("status", status)),
	)
}

// --- Authz Check ---

func (o *metricsObserver) AuthzCheckStarted(
	ctx context.Context,
) (context.Context, service.AuthzCheckProbe) {
	return ctx, &metricsAuthzCheckProbe{
		total:              o.authzCheckTotal,
		duration:           o.authzCheckDuration,
		validationFailures: o.authzCheckValidationFailures,
		start:              time.Now(),
	}
}

type metricsAuthzCheckProbe struct {
	service.NoOpAuthzCheckProbe
	total              metric.Int64Counter
	duration           metric.Float64Histogram
	validationFailures metric.Int64Counter
	start              time.Time
	mu                 sync.Mutex
	failed             bool
}

func (p *metricsAuthzCheckProbe) ActorValidationFailed(_ error) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
	p.validationFailures.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("stage", "actor")),
	)
}

func (p *metricsAuthzCheckProbe) SubjectValidationFailed(_ error) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
	p.validationFailures.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("stage", "subject")),
	)
}

func (p *metricsAuthzCheckProbe) SubjectCredentialExtractionFailed(_ error) {
	p.mu.Lock()
	p.failed = true
	p.mu.Unlock()
	p.validationFailures.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("stage", "subject_extraction")),
	)
}

func (p *metricsAuthzCheckProbe) End() {
	p.mu.Lock()
	failed := p.failed
	p.mu.Unlock()

	status := "success"
	if failed {
		status = "failure"
	}
	p.total.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("status", status)),
	)
	p.duration.Record(context.Background(), time.Since(p.start).Seconds(),
		metric.WithAttributes(attribute.String("status", status)),
	)
}
