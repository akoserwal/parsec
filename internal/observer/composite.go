package observer

import (
	"context"

	"github.com/project-kessel/parsec/internal/datasource"
	"github.com/project-kessel/parsec/internal/keys"
	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

// CompositeAll builds an Observer that fans out every call to all children.
// ApplicationObserver fan-out uses service.NewCompositeObserver (handles
// request-scoped probes with context); infra fan-out is handled here.
func CompositeAll(children []Observer) Observer {
	if len(children) == 1 {
		return children[0]
	}
	apps := make([]service.ApplicationObserver, len(children))
	for i, c := range children {
		apps[i] = c
	}
	return &compositeAll{
		ApplicationObserver: service.NewCompositeObserver(apps...),
		children:            children,
	}
}

type compositeAll struct {
	service.ApplicationObserver
	children []Observer
}

// --- probe factories: each creates probes from all children and wraps them ---

func (c *compositeAll) DataSourceCacheProbe(ctx context.Context, name string) datasource.DataSourceCacheProbe {
	probes := make([]datasource.DataSourceCacheProbe, len(c.children))
	for i, ch := range c.children {
		probes[i] = ch.DataSourceCacheProbe(ctx, name)
	}
	return &multiDSCacheProbe{probes}
}

func (c *compositeAll) LuaDataSourceProbe(ctx context.Context, name string) datasource.LuaDataSourceProbe {
	probes := make([]datasource.LuaDataSourceProbe, len(c.children))
	for i, ch := range c.children {
		probes[i] = ch.LuaDataSourceProbe(ctx, name)
	}
	return &multiLuaDSProbe{probes}
}

func (c *compositeAll) KeyRotationProbe(ctx context.Context) keys.KeyRotationProbe {
	probes := make([]keys.KeyRotationProbe, len(c.children))
	for i, ch := range c.children {
		probes[i] = ch.KeyRotationProbe(ctx)
	}
	return &multiKeyRotProbe{probes}
}

func (c *compositeAll) KeyProviderProbe(ctx context.Context) keys.KeyProviderProbe {
	probes := make([]keys.KeyProviderProbe, len(c.children))
	for i, ch := range c.children {
		probes[i] = ch.KeyProviderProbe(ctx)
	}
	return &multiKeyProvProbe{probes}
}

func (c *compositeAll) TrustValidationProbe(ctx context.Context) trust.TrustValidationProbe {
	probes := make([]trust.TrustValidationProbe, len(c.children))
	for i, ch := range c.children {
		probes[i] = ch.TrustValidationProbe(ctx)
	}
	return &multiTrustProbe{probes}
}

func (c *compositeAll) JWKSCacheProbe(ctx context.Context) server.JWKSCacheProbe {
	probes := make([]server.JWKSCacheProbe, len(c.children))
	for i, ch := range c.children {
		probes[i] = ch.JWKSCacheProbe(ctx)
	}
	return &multiJWKSProbe{probes}
}

func (c *compositeAll) ServerLifecycleProbe(ctx context.Context) server.ServerLifecycleProbe {
	probes := make([]server.ServerLifecycleProbe, len(c.children))
	for i, ch := range c.children {
		probes[i] = ch.ServerLifecycleProbe(ctx)
	}
	return &multiSrvLifeProbe{probes}
}

// --- composite probes: fan out each event to all children ---

type multiDSCacheProbe struct {
	probes []datasource.DataSourceCacheProbe
}

func (m *multiDSCacheProbe) CacheHit() {
	for _, p := range m.probes {
		p.CacheHit()
	}
}
func (m *multiDSCacheProbe) CacheMiss() {
	for _, p := range m.probes {
		p.CacheMiss()
	}
}
func (m *multiDSCacheProbe) CacheExpired() {
	for _, p := range m.probes {
		p.CacheExpired()
	}
}
func (m *multiDSCacheProbe) FetchFailed(err error) {
	for _, p := range m.probes {
		p.FetchFailed(err)
	}
}

type multiLuaDSProbe struct {
	probes []datasource.LuaDataSourceProbe
}

func (m *multiLuaDSProbe) ScriptLoadFailed(err error) {
	for _, p := range m.probes {
		p.ScriptLoadFailed(err)
	}
}
func (m *multiLuaDSProbe) ScriptExecutionFailed(err error) {
	for _, p := range m.probes {
		p.ScriptExecutionFailed(err)
	}
}
func (m *multiLuaDSProbe) InvalidReturnType(got string) {
	for _, p := range m.probes {
		p.InvalidReturnType(got)
	}
}
func (m *multiLuaDSProbe) FetchCompleted() {
	for _, p := range m.probes {
		p.FetchCompleted()
	}
}

type multiKeyRotProbe struct{ probes []keys.KeyRotationProbe }

func (m *multiKeyRotProbe) RotationCheckFailed(err error) {
	for _, p := range m.probes {
		p.RotationCheckFailed(err)
	}
}
func (m *multiKeyRotProbe) ActiveKeyCacheUpdateFailed(err error) {
	for _, p := range m.probes {
		p.ActiveKeyCacheUpdateFailed(err)
	}
}
func (m *multiKeyRotProbe) RotationCompleted(slot string) {
	for _, p := range m.probes {
		p.RotationCompleted(slot)
	}
}
func (m *multiKeyRotProbe) RotationSkippedVersionRace(slot string) {
	for _, p := range m.probes {
		p.RotationSkippedVersionRace(slot)
	}
}
func (m *multiKeyRotProbe) KeyProviderNotFound(prov, slot string) {
	for _, p := range m.probes {
		p.KeyProviderNotFound(prov, slot)
	}
}
func (m *multiKeyRotProbe) KeyHandleFailed(slot string, err error) {
	for _, p := range m.probes {
		p.KeyHandleFailed(slot, err)
	}
}
func (m *multiKeyRotProbe) PublicKeyFailed(slot string, err error) {
	for _, p := range m.probes {
		p.PublicKeyFailed(slot, err)
	}
}
func (m *multiKeyRotProbe) ThumbprintFailed(slot string, err error) {
	for _, p := range m.probes {
		p.ThumbprintFailed(slot, err)
	}
}
func (m *multiKeyRotProbe) MetadataFailed(slot string, err error) {
	for _, p := range m.probes {
		p.MetadataFailed(slot, err)
	}
}

type multiKeyProvProbe struct{ probes []keys.KeyProviderProbe }

func (m *multiKeyProvProbe) OldKeyDeletionFailed(keyID string, err error) {
	for _, p := range m.probes {
		p.OldKeyDeletionFailed(keyID, err)
	}
}

type multiTrustProbe struct{ probes []trust.TrustValidationProbe }

func (m *multiTrustProbe) ValidatorFailed(name string, ct trust.CredentialType, err error) {
	for _, p := range m.probes {
		p.ValidatorFailed(name, ct, err)
	}
}
func (m *multiTrustProbe) AllValidatorsFailed(ct trust.CredentialType, n int, err error) {
	for _, p := range m.probes {
		p.AllValidatorsFailed(ct, n, err)
	}
}
func (m *multiTrustProbe) ValidatorFiltered(name, actor string) {
	for _, p := range m.probes {
		p.ValidatorFiltered(name, actor)
	}
}
func (m *multiTrustProbe) FilterEvaluationFailed(name string, err error) {
	for _, p := range m.probes {
		p.FilterEvaluationFailed(name, err)
	}
}

type multiJWKSProbe struct{ probes []server.JWKSCacheProbe }

func (m *multiJWKSProbe) InitialCachePopulationFailed(err error) {
	for _, p := range m.probes {
		p.InitialCachePopulationFailed(err)
	}
}
func (m *multiJWKSProbe) CacheRefreshFailed(err error) {
	for _, p := range m.probes {
		p.CacheRefreshFailed(err)
	}
}
func (m *multiJWKSProbe) KeyConversionFailed(keyID string, err error) {
	for _, p := range m.probes {
		p.KeyConversionFailed(keyID, err)
	}
}

type multiSrvLifeProbe struct{ probes []server.ServerLifecycleProbe }

func (m *multiSrvLifeProbe) GRPCServeFailed(err error) {
	for _, p := range m.probes {
		p.GRPCServeFailed(err)
	}
}
func (m *multiSrvLifeProbe) HTTPServeFailed(err error) {
	for _, p := range m.probes {
		p.HTTPServeFailed(err)
	}
}

var _ Observer = (*compositeAll)(nil)
