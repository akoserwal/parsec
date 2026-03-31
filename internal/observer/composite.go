package observer

import (
	"context"

	"github.com/project-kessel/parsec/internal/datasource"
	"github.com/project-kessel/parsec/internal/keys"
	"github.com/project-kessel/parsec/internal/request"
	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

// CompositeAll builds an Observer that fans out every call to all children.
func CompositeAll(children []Observer) Observer {
	if len(children) == 1 {
		return children[0]
	}
	return &compositeAll{children: children}
}

type compositeAll struct {
	children []Observer
}

// --- probe factories: each creates probes from all children and wraps them ---

func (c *compositeAll) TokenIssuanceStarted(
	ctx context.Context,
	subject *trust.Result,
	actor *trust.Result,
	scope string,
	tokenTypes []service.TokenType,
) (context.Context, service.TokenIssuanceProbe) {
	probes := make([]service.TokenIssuanceProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.TokenIssuanceStarted(ctx, subject, actor, scope, tokenTypes)
	}
	return ctx, &compositeTokenIssuanceProbe{probes: probes}
}

func (c *compositeAll) TokenExchangeStarted(
	ctx context.Context,
	grantType string,
	requestedTokenType string,
	audience string,
	scope string,
) (context.Context, service.TokenExchangeProbe) {
	probes := make([]service.TokenExchangeProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.TokenExchangeStarted(ctx, grantType, requestedTokenType, audience, scope)
	}
	return ctx, &compositeTokenExchangeProbe{probes: probes}
}

func (c *compositeAll) AuthzCheckStarted(
	ctx context.Context,
) (context.Context, service.AuthzCheckProbe) {
	probes := make([]service.AuthzCheckProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.AuthzCheckStarted(ctx)
	}
	return ctx, &compositeAuthzCheckProbe{probes: probes}
}

func (c *compositeAll) CacheFetchStarted(ctx context.Context, name string) (context.Context, datasource.CacheFetchProbe) {
	probes := make([]datasource.CacheFetchProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.CacheFetchStarted(ctx, name)
	}
	return ctx, &compositeCacheFetchProbe{probes}
}

func (c *compositeAll) LuaFetchStarted(ctx context.Context, name string) (context.Context, datasource.LuaFetchProbe) {
	probes := make([]datasource.LuaFetchProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.LuaFetchStarted(ctx, name)
	}
	return ctx, &compositeLuaFetchProbe{probes}
}

func (c *compositeAll) RotationCheckStarted(ctx context.Context) (context.Context, keys.RotationCheckProbe) {
	probes := make([]keys.RotationCheckProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.RotationCheckStarted(ctx)
	}
	return ctx, &compositeRotationCheckProbe{probes}
}

func (c *compositeAll) KeyProvisionStarted(ctx context.Context) (context.Context, keys.KeyProvisionProbe) {
	probes := make([]keys.KeyProvisionProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.KeyProvisionStarted(ctx)
	}
	return ctx, &compositeKeyProvisionProbe{probes}
}

func (c *compositeAll) ValidationStarted(ctx context.Context) (context.Context, trust.ValidationProbe) {
	probes := make([]trust.ValidationProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.ValidationStarted(ctx)
	}
	return ctx, &compositeValidationProbe{probes}
}

func (c *compositeAll) CacheRefreshStarted(ctx context.Context) (context.Context, server.CacheRefreshProbe) {
	probes := make([]server.CacheRefreshProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.CacheRefreshStarted(ctx)
	}
	return ctx, &compositeCacheRefreshProbe{probes}
}

func (c *compositeAll) ServeStarted(ctx context.Context) (context.Context, server.ServeProbe) {
	probes := make([]server.ServeProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.ServeStarted(ctx)
	}
	return ctx, &compositeServeProbe{probes}
}

// --- composite probes: fan out each event to all children ---

type compositeCacheFetchProbe struct {
	probes []datasource.CacheFetchProbe
}

func (m *compositeCacheFetchProbe) CacheHit() {
	for _, p := range m.probes {
		p.CacheHit()
	}
}
func (m *compositeCacheFetchProbe) CacheMiss() {
	for _, p := range m.probes {
		p.CacheMiss()
	}
}
func (m *compositeCacheFetchProbe) CacheExpired() {
	for _, p := range m.probes {
		p.CacheExpired()
	}
}
func (m *compositeCacheFetchProbe) FetchFailed(err error) {
	for _, p := range m.probes {
		p.FetchFailed(err)
	}
}
func (m *compositeCacheFetchProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeLuaFetchProbe struct {
	probes []datasource.LuaFetchProbe
}

func (m *compositeLuaFetchProbe) ScriptLoadFailed(err error) {
	for _, p := range m.probes {
		p.ScriptLoadFailed(err)
	}
}
func (m *compositeLuaFetchProbe) ScriptExecutionFailed(err error) {
	for _, p := range m.probes {
		p.ScriptExecutionFailed(err)
	}
}
func (m *compositeLuaFetchProbe) InvalidReturnType(got string) {
	for _, p := range m.probes {
		p.InvalidReturnType(got)
	}
}
func (m *compositeLuaFetchProbe) FetchCompleted() {
	for _, p := range m.probes {
		p.FetchCompleted()
	}
}
func (m *compositeLuaFetchProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeRotationCheckProbe struct{ probes []keys.RotationCheckProbe }

func (m *compositeRotationCheckProbe) RotationCheckFailed(err error) {
	for _, p := range m.probes {
		p.RotationCheckFailed(err)
	}
}
func (m *compositeRotationCheckProbe) ActiveKeyCacheUpdateFailed(err error) {
	for _, p := range m.probes {
		p.ActiveKeyCacheUpdateFailed(err)
	}
}
func (m *compositeRotationCheckProbe) RotationCompleted(slot string) {
	for _, p := range m.probes {
		p.RotationCompleted(slot)
	}
}
func (m *compositeRotationCheckProbe) RotationSkippedVersionRace(slot string) {
	for _, p := range m.probes {
		p.RotationSkippedVersionRace(slot)
	}
}
func (m *compositeRotationCheckProbe) KeyProviderNotFound(prov, slot string) {
	for _, p := range m.probes {
		p.KeyProviderNotFound(prov, slot)
	}
}
func (m *compositeRotationCheckProbe) KeyHandleFailed(slot string, err error) {
	for _, p := range m.probes {
		p.KeyHandleFailed(slot, err)
	}
}
func (m *compositeRotationCheckProbe) PublicKeyFailed(slot string, err error) {
	for _, p := range m.probes {
		p.PublicKeyFailed(slot, err)
	}
}
func (m *compositeRotationCheckProbe) ThumbprintFailed(slot string, err error) {
	for _, p := range m.probes {
		p.ThumbprintFailed(slot, err)
	}
}
func (m *compositeRotationCheckProbe) MetadataFailed(slot string, err error) {
	for _, p := range m.probes {
		p.MetadataFailed(slot, err)
	}
}
func (m *compositeRotationCheckProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeKeyProvisionProbe struct{ probes []keys.KeyProvisionProbe }

func (m *compositeKeyProvisionProbe) OldKeyDeletionFailed(keyID string, err error) {
	for _, p := range m.probes {
		p.OldKeyDeletionFailed(keyID, err)
	}
}
func (m *compositeKeyProvisionProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeValidationProbe struct{ probes []trust.ValidationProbe }

func (m *compositeValidationProbe) ValidatorFailed(name string, ct trust.CredentialType, err error) {
	for _, p := range m.probes {
		p.ValidatorFailed(name, ct, err)
	}
}
func (m *compositeValidationProbe) AllValidatorsFailed(ct trust.CredentialType, n int, err error) {
	for _, p := range m.probes {
		p.AllValidatorsFailed(ct, n, err)
	}
}
func (m *compositeValidationProbe) ValidatorFiltered(name, actor string) {
	for _, p := range m.probes {
		p.ValidatorFiltered(name, actor)
	}
}
func (m *compositeValidationProbe) FilterEvaluationFailed(name string, err error) {
	for _, p := range m.probes {
		p.FilterEvaluationFailed(name, err)
	}
}
func (m *compositeValidationProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeCacheRefreshProbe struct{ probes []server.CacheRefreshProbe }

func (m *compositeCacheRefreshProbe) InitialCachePopulationFailed(err error) {
	for _, p := range m.probes {
		p.InitialCachePopulationFailed(err)
	}
}
func (m *compositeCacheRefreshProbe) CacheRefreshFailed(err error) {
	for _, p := range m.probes {
		p.CacheRefreshFailed(err)
	}
}
func (m *compositeCacheRefreshProbe) KeyConversionFailed(keyID string, err error) {
	for _, p := range m.probes {
		p.KeyConversionFailed(keyID, err)
	}
}
func (m *compositeCacheRefreshProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeServeProbe struct{ probes []server.ServeProbe }

func (m *compositeServeProbe) GRPCServeFailed(err error) {
	for _, p := range m.probes {
		p.GRPCServeFailed(err)
	}
}
func (m *compositeServeProbe) HTTPServeFailed(err error) {
	for _, p := range m.probes {
		p.HTTPServeFailed(err)
	}
}
func (m *compositeServeProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeTokenIssuanceProbe struct {
	probes []service.TokenIssuanceProbe
}

func (m *compositeTokenIssuanceProbe) TokenTypeIssuanceStarted(tokenType service.TokenType) {
	for _, p := range m.probes {
		p.TokenTypeIssuanceStarted(tokenType)
	}
}

func (m *compositeTokenIssuanceProbe) TokenTypeIssuanceSucceeded(tokenType service.TokenType, token *service.Token) {
	for _, p := range m.probes {
		p.TokenTypeIssuanceSucceeded(tokenType, token)
	}
}

func (m *compositeTokenIssuanceProbe) TokenTypeIssuanceFailed(tokenType service.TokenType, err error) {
	for _, p := range m.probes {
		p.TokenTypeIssuanceFailed(tokenType, err)
	}
}

func (m *compositeTokenIssuanceProbe) IssuerNotFound(tokenType service.TokenType, err error) {
	for _, p := range m.probes {
		p.IssuerNotFound(tokenType, err)
	}
}

func (m *compositeTokenIssuanceProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeTokenExchangeProbe struct {
	probes []service.TokenExchangeProbe
}

func (m *compositeTokenExchangeProbe) ActorValidationSucceeded(actor *trust.Result) {
	for _, p := range m.probes {
		p.ActorValidationSucceeded(actor)
	}
}

func (m *compositeTokenExchangeProbe) ActorValidationFailed(err error) {
	for _, p := range m.probes {
		p.ActorValidationFailed(err)
	}
}

func (m *compositeTokenExchangeProbe) RequestContextParsed(attrs *request.RequestAttributes) {
	for _, p := range m.probes {
		p.RequestContextParsed(attrs)
	}
}

func (m *compositeTokenExchangeProbe) RequestContextParseFailed(err error) {
	for _, p := range m.probes {
		p.RequestContextParseFailed(err)
	}
}

func (m *compositeTokenExchangeProbe) SubjectTokenValidationSucceeded(subject *trust.Result) {
	for _, p := range m.probes {
		p.SubjectTokenValidationSucceeded(subject)
	}
}

func (m *compositeTokenExchangeProbe) SubjectTokenValidationFailed(err error) {
	for _, p := range m.probes {
		p.SubjectTokenValidationFailed(err)
	}
}

func (m *compositeTokenExchangeProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type compositeAuthzCheckProbe struct {
	probes []service.AuthzCheckProbe
}

func (m *compositeAuthzCheckProbe) RequestAttributesParsed(attrs *request.RequestAttributes) {
	for _, p := range m.probes {
		p.RequestAttributesParsed(attrs)
	}
}

func (m *compositeAuthzCheckProbe) ActorValidationSucceeded(actor *trust.Result) {
	for _, p := range m.probes {
		p.ActorValidationSucceeded(actor)
	}
}

func (m *compositeAuthzCheckProbe) ActorValidationFailed(err error) {
	for _, p := range m.probes {
		p.ActorValidationFailed(err)
	}
}

func (m *compositeAuthzCheckProbe) SubjectCredentialExtracted(cred trust.Credential, headersUsed []string) {
	for _, p := range m.probes {
		p.SubjectCredentialExtracted(cred, headersUsed)
	}
}

func (m *compositeAuthzCheckProbe) SubjectCredentialExtractionFailed(err error) {
	for _, p := range m.probes {
		p.SubjectCredentialExtractionFailed(err)
	}
}

func (m *compositeAuthzCheckProbe) SubjectValidationSucceeded(subject *trust.Result) {
	for _, p := range m.probes {
		p.SubjectValidationSucceeded(subject)
	}
}

func (m *compositeAuthzCheckProbe) SubjectValidationFailed(err error) {
	for _, p := range m.probes {
		p.SubjectValidationFailed(err)
	}
}

func (m *compositeAuthzCheckProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

var _ Observer = (*compositeAll)(nil)
