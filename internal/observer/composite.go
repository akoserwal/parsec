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

func (c *compositeAll) CacheFetchStarted(ctx context.Context, name string) (context.Context, datasource.CacheFetchProbe) {
	probes := make([]datasource.CacheFetchProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.CacheFetchStarted(ctx, name)
	}
	return ctx, &multiCacheFetchProbe{probes}
}

func (c *compositeAll) LuaFetchStarted(ctx context.Context, name string) (context.Context, datasource.LuaFetchProbe) {
	probes := make([]datasource.LuaFetchProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.LuaFetchStarted(ctx, name)
	}
	return ctx, &multiLuaFetchProbe{probes}
}

func (c *compositeAll) RotationCheckStarted(ctx context.Context) (context.Context, keys.RotationCheckProbe) {
	probes := make([]keys.RotationCheckProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.RotationCheckStarted(ctx)
	}
	return ctx, &multiRotationCheckProbe{probes}
}

func (c *compositeAll) KeyProvisionStarted(ctx context.Context) (context.Context, keys.KeyProvisionProbe) {
	probes := make([]keys.KeyProvisionProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.KeyProvisionStarted(ctx)
	}
	return ctx, &multiKeyProvisionProbe{probes}
}

func (c *compositeAll) ValidationStarted(ctx context.Context) (context.Context, trust.ValidationProbe) {
	probes := make([]trust.ValidationProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.ValidationStarted(ctx)
	}
	return ctx, &multiValidationProbe{probes}
}

func (c *compositeAll) CacheRefreshStarted(ctx context.Context) (context.Context, server.CacheRefreshProbe) {
	probes := make([]server.CacheRefreshProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.CacheRefreshStarted(ctx)
	}
	return ctx, &multiCacheRefreshProbe{probes}
}

func (c *compositeAll) ServeStarted(ctx context.Context) (context.Context, server.ServeProbe) {
	probes := make([]server.ServeProbe, len(c.children))
	for i, ch := range c.children {
		ctx, probes[i] = ch.ServeStarted(ctx)
	}
	return ctx, &multiServeProbe{probes}
}

// --- composite probes: fan out each event to all children ---

type multiCacheFetchProbe struct {
	probes []datasource.CacheFetchProbe
}

func (m *multiCacheFetchProbe) CacheHit() {
	for _, p := range m.probes {
		p.CacheHit()
	}
}
func (m *multiCacheFetchProbe) CacheMiss() {
	for _, p := range m.probes {
		p.CacheMiss()
	}
}
func (m *multiCacheFetchProbe) CacheExpired() {
	for _, p := range m.probes {
		p.CacheExpired()
	}
}
func (m *multiCacheFetchProbe) FetchFailed(err error) {
	for _, p := range m.probes {
		p.FetchFailed(err)
	}
}
func (m *multiCacheFetchProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type multiLuaFetchProbe struct {
	probes []datasource.LuaFetchProbe
}

func (m *multiLuaFetchProbe) ScriptLoadFailed(err error) {
	for _, p := range m.probes {
		p.ScriptLoadFailed(err)
	}
}
func (m *multiLuaFetchProbe) ScriptExecutionFailed(err error) {
	for _, p := range m.probes {
		p.ScriptExecutionFailed(err)
	}
}
func (m *multiLuaFetchProbe) InvalidReturnType(got string) {
	for _, p := range m.probes {
		p.InvalidReturnType(got)
	}
}
func (m *multiLuaFetchProbe) FetchCompleted() {
	for _, p := range m.probes {
		p.FetchCompleted()
	}
}
func (m *multiLuaFetchProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type multiRotationCheckProbe struct{ probes []keys.RotationCheckProbe }

func (m *multiRotationCheckProbe) RotationCheckFailed(err error) {
	for _, p := range m.probes {
		p.RotationCheckFailed(err)
	}
}
func (m *multiRotationCheckProbe) ActiveKeyCacheUpdateFailed(err error) {
	for _, p := range m.probes {
		p.ActiveKeyCacheUpdateFailed(err)
	}
}
func (m *multiRotationCheckProbe) RotationCompleted(slot string) {
	for _, p := range m.probes {
		p.RotationCompleted(slot)
	}
}
func (m *multiRotationCheckProbe) RotationSkippedVersionRace(slot string) {
	for _, p := range m.probes {
		p.RotationSkippedVersionRace(slot)
	}
}
func (m *multiRotationCheckProbe) KeyProviderNotFound(prov, slot string) {
	for _, p := range m.probes {
		p.KeyProviderNotFound(prov, slot)
	}
}
func (m *multiRotationCheckProbe) KeyHandleFailed(slot string, err error) {
	for _, p := range m.probes {
		p.KeyHandleFailed(slot, err)
	}
}
func (m *multiRotationCheckProbe) PublicKeyFailed(slot string, err error) {
	for _, p := range m.probes {
		p.PublicKeyFailed(slot, err)
	}
}
func (m *multiRotationCheckProbe) ThumbprintFailed(slot string, err error) {
	for _, p := range m.probes {
		p.ThumbprintFailed(slot, err)
	}
}
func (m *multiRotationCheckProbe) MetadataFailed(slot string, err error) {
	for _, p := range m.probes {
		p.MetadataFailed(slot, err)
	}
}
func (m *multiRotationCheckProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type multiKeyProvisionProbe struct{ probes []keys.KeyProvisionProbe }

func (m *multiKeyProvisionProbe) OldKeyDeletionFailed(keyID string, err error) {
	for _, p := range m.probes {
		p.OldKeyDeletionFailed(keyID, err)
	}
}
func (m *multiKeyProvisionProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type multiValidationProbe struct{ probes []trust.ValidationProbe }

func (m *multiValidationProbe) ValidatorFailed(name string, ct trust.CredentialType, err error) {
	for _, p := range m.probes {
		p.ValidatorFailed(name, ct, err)
	}
}
func (m *multiValidationProbe) AllValidatorsFailed(ct trust.CredentialType, n int, err error) {
	for _, p := range m.probes {
		p.AllValidatorsFailed(ct, n, err)
	}
}
func (m *multiValidationProbe) ValidatorFiltered(name, actor string) {
	for _, p := range m.probes {
		p.ValidatorFiltered(name, actor)
	}
}
func (m *multiValidationProbe) FilterEvaluationFailed(name string, err error) {
	for _, p := range m.probes {
		p.FilterEvaluationFailed(name, err)
	}
}
func (m *multiValidationProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type multiCacheRefreshProbe struct{ probes []server.CacheRefreshProbe }

func (m *multiCacheRefreshProbe) InitialCachePopulationFailed(err error) {
	for _, p := range m.probes {
		p.InitialCachePopulationFailed(err)
	}
}
func (m *multiCacheRefreshProbe) CacheRefreshFailed(err error) {
	for _, p := range m.probes {
		p.CacheRefreshFailed(err)
	}
}
func (m *multiCacheRefreshProbe) KeyConversionFailed(keyID string, err error) {
	for _, p := range m.probes {
		p.KeyConversionFailed(keyID, err)
	}
}
func (m *multiCacheRefreshProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

type multiServeProbe struct{ probes []server.ServeProbe }

func (m *multiServeProbe) GRPCServeFailed(err error) {
	for _, p := range m.probes {
		p.GRPCServeFailed(err)
	}
}
func (m *multiServeProbe) HTTPServeFailed(err error) {
	for _, p := range m.probes {
		p.HTTPServeFailed(err)
	}
}
func (m *multiServeProbe) End() {
	for _, p := range m.probes {
		p.End()
	}
}

var _ Observer = (*compositeAll)(nil)
